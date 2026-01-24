package opencode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// RunOptions configures an opencode run.
type RunOptions struct {
	RepoPath  string
	WorkDir   string
	Prompt    string
	StartedAt time.Time
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Env       []string
}

// RunResult captures output from running opencode.
type RunResult struct {
	SessionID string
	ExitCode  int
}

// Run executes opencode and records session state.
func (s *Store) Run(opts RunOptions) (*RunHandle, error) {
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	repoPath := opts.RepoPath
	if repoPath == "" {
		repoPath = opts.WorkDir
	}
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = repoPath
	}

	env := opts.Env
	if env == nil {
		env = os.Environ()
	}
	if workDir != "" {
		env = replaceEnvVar(env, "PWD", workDir)
	}

	runStdout := opts.Stdout
	if runStdout == nil {
		runStdout = os.Stdout
	}
	runStderr := opts.Stderr
	if runStderr == nil {
		runStderr = os.Stderr
	}
	runStdin := resolveRunStdin(opts)

	port, err := allocatePort()
	if err != nil {
		return nil, err
	}

	serverHost := "127.0.0.1"
	serverURL := fmt.Sprintf("http://%s:%d", serverHost, port)
	eventURL := serverURL + "/event"

	serveCmd := exec.Command("opencode", "serve", "--port="+strconv.Itoa(port), "--hostname="+serverHost)
	serveCmd.Dir = workDir
	serveCmd.Env = env
	serveCmd.Stdout = runStderr
	serveCmd.Stderr = runStderr

	if err := serveCmd.Start(); err != nil {
		return nil, err
	}

	resp, err := connectEventStream(eventURL, 5*time.Second)
	if err != nil {
		stopErr := stopServeCommand(serveCmd)
		return nil, errors.Join(err, stopErr)
	}

	recorder, err := s.events.newRecorder()
	if err != nil {
		_ = resp.Body.Close()
		stopErr := stopServeCommand(serveCmd)
		return nil, errors.Join(err, stopErr)
	}

	events := make(chan Event, 32)
	eventCtx, cancelEvents := context.WithCancel(context.Background())
	eventErrCh := make(chan error, 1)
	go func() {
		eventErrCh <- readEventStream(eventCtx, resp.Body, recorder, events)
		close(events)
	}()

	sessionCh := make(chan sessionResult, 1)
	go func() {
		session, err := s.ensureSession(repoPath, startedAt, opts.Prompt)
		var recordErr error
		if err == nil {
			recordErr = recorder.SetSessionID(session.ID)
		}
		sessionCh <- sessionResult{session: session, err: err, recordErr: recordErr}
	}()

	runCmd := exec.Command("opencode", "run", "--attach="+serverURL)
	runCmd.Dir = workDir
	runCmd.Env = env
	runCmd.Stdout = runStdout
	runCmd.Stderr = runStderr
	runCmd.Stdin = runStdin

	if err := runCmd.Start(); err != nil {
		cancelEvents()
		_ = resp.Body.Close()
		stopErr := stopServeCommand(serveCmd)
		return nil, errors.Join(err, stopErr)
	}

	handle := &RunHandle{
		Events: events,
		wait: func() (RunResult, error) {
			exitCode, runErr := runExitCode(runCmd)
			completedAt := time.Now()

			sessionResult := <-sessionCh
			if sessionResult.err != nil {
				runErr = errors.Join(runErr, sessionResult.err)
			}
			if sessionResult.recordErr != nil {
				runErr = errors.Join(runErr, sessionResult.recordErr)
			}

			cancelEvents()
			_ = resp.Body.Close()
			eventErr := <-eventErrCh
			stopErr := stopServeCommand(serveCmd)
			if eventErr != nil {
				runErr = errors.Join(runErr, eventErr)
			}
			if stopErr != nil {
				runErr = errors.Join(runErr, stopErr)
			}

			if sessionResult.err == nil {
				duration := int(completedAt.Sub(sessionResult.session.StartedAt).Seconds())
				status := OpencodeSessionCompleted
				if exitCode != 0 {
					status = OpencodeSessionFailed
				}
				if _, err := s.CompleteSession(repoPath, sessionResult.session.ID, status, completedAt, &exitCode, duration); err != nil {
					runErr = errors.Join(runErr, err)
				}
			}

			if runErr != nil {
				return RunResult{}, runErr
			}
			return RunResult{SessionID: sessionResult.session.ID, ExitCode: exitCode}, nil
		},
	}

	return handle, nil
}

func (s *Store) ensureSession(repoPath string, startedAt time.Time, prompt string) (OpencodeSession, error) {
	metadata, err := s.storage.FindSessionForRunWithRetry(repoPath, startedAt, prompt, 5*time.Second)
	if err != nil {
		return OpencodeSession{}, err
	}

	sessionStartedAt := startedAt
	if !metadata.CreatedAt.IsZero() {
		sessionStartedAt = metadata.CreatedAt
	}

	if existing, err := s.FindSession(repoPath, metadata.ID); err == nil {
		if existing.Status == OpencodeSessionActive {
			return existing, nil
		}
	} else if !errors.Is(err, ErrOpencodeSessionNotFound) {
		return OpencodeSession{}, err
	}

	return s.CreateSession(repoPath, metadata.ID, prompt, sessionStartedAt)
}

func replaceEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	updated := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		updated = append(updated, entry)
	}
	updated = append(updated, prefix+value)
	return updated
}

func resolveRunStdin(opts RunOptions) io.Reader {
	if opts.Stdin != nil {
		return opts.Stdin
	}
	if opts.Prompt != "" {
		return strings.NewReader(opts.Prompt)
	}
	return os.Stdin
}

func runExitCode(cmd *exec.Cmd) (int, error) {
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

type sessionResult struct {
	session   OpencodeSession
	err       error
	recordErr error
}

func allocatePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen for opencode port: %w", err)
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address: %T", listener.Addr())
	}
	return addr.Port, nil
}

func connectEventStream(url string, timeout time.Duration) (*http.Response, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, lastErr
		}

		transport := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: remaining}).DialContext,
			ResponseHeaderTimeout: remaining,
		}
		client := &http.Client{Transport: transport}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				return resp, nil
			}
			lastErr = fmt.Errorf("unexpected event status: %s", resp.Status)
			_ = resp.Body.Close()
		} else {
			lastErr = err
		}
		transport.CloseIdleConnections()
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func stopServeCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	signalErr := cmd.Process.Signal(os.Interrupt)
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case waitErr := <-waitCh:
		if errors.Is(waitErr, os.ErrProcessDone) {
			waitErr = nil
		}
		if isExpectedServeExit(waitErr) {
			waitErr = nil
		}
		return errors.Join(signalErr, waitErr)
	case <-time.After(2 * time.Second):
		killErr := cmd.Process.Kill()
		waitErr := <-waitCh
		if errors.Is(waitErr, os.ErrProcessDone) {
			waitErr = nil
		}
		if isExpectedServeExit(waitErr) {
			waitErr = nil
		}
		return errors.Join(signalErr, killErr, waitErr)
	}
}

func isExpectedServeExit(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}
