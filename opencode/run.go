package opencode

import (
	"errors"
	"io"
	"os"
	"os/exec"
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
func (s *Store) Run(opts RunOptions) (RunResult, error) {
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	repoPath := opts.RepoPath
	if repoPath == "" {
		repoPath = opts.WorkDir
	}

	runCmd := exec.Command("opencode", "run", opts.Prompt)
	runCmd.Dir = opts.WorkDir
	if runCmd.Dir == "" {
		runCmd.Dir = repoPath
	}

	runCmd.Env = opts.Env
	if runCmd.Env == nil {
		runCmd.Env = os.Environ()
	}
	if runCmd.Dir != "" {
		runCmd.Env = replaceEnvVar(runCmd.Env, "PWD", runCmd.Dir)
	}

	runCmd.Stdout = opts.Stdout
	if runCmd.Stdout == nil {
		runCmd.Stdout = os.Stdout
	}
	runCmd.Stderr = opts.Stderr
	if runCmd.Stderr == nil {
		runCmd.Stderr = os.Stderr
	}
	runCmd.Stdin = opts.Stdin
	if runCmd.Stdin == nil {
		runCmd.Stdin = os.Stdin
	}

	if err := runCmd.Start(); err != nil {
		return RunResult{}, err
	}

	session, sessionErr := s.ensureSession(repoPath, startedAt, opts.Prompt)
	exitCode, runErr := runExitCode(runCmd)
	completedAt := time.Now()

	if sessionErr != nil {
		session, sessionErr = s.ensureSession(repoPath, startedAt, opts.Prompt)
	}
	if sessionErr != nil {
		if runErr != nil {
			return RunResult{}, errors.Join(runErr, sessionErr)
		}
		return RunResult{}, sessionErr
	}

	duration := int(completedAt.Sub(session.StartedAt).Seconds())
	status := OpencodeSessionCompleted
	if exitCode != 0 {
		status = OpencodeSessionFailed
	}
	if _, err := s.CompleteSession(repoPath, session.ID, status, completedAt, &exitCode, duration); err != nil {
		return RunResult{}, err
	}
	if runErr != nil {
		return RunResult{}, runErr
	}
	return RunResult{SessionID: session.ID, ExitCode: exitCode}, nil
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
