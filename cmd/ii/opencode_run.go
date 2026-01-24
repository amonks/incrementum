package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	internalopencode "github.com/amonks/incrementum/internal/opencode"
	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

var opencodeRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Start a new opencode session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runOpencodeRun,
}

const opencodeSessionLookupTimeout = 5 * time.Second

func init() {
	opencodeCmd.AddCommand(opencodeRunCmd)
}

func runOpencodeRun(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getOpencodeRepoPath()
	if err != nil {
		return err
	}

	prompt, err := resolveOpencodePrompt(args, os.Stdin)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	storage, err := opencodeStorage()
	if err != nil {
		return err
	}

	runCmd := exec.Command("opencode", "run", prompt)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin
	if err := runCmd.Start(); err != nil {
		return err
	}

	session, sessionErr := ensureOpencodeSession(pool, storage, repoPath, startedAt, prompt)
	exitCode, runErr := runExitCode(runCmd)
	completedAt := time.Now()
	if sessionErr != nil {
		session, sessionErr = ensureOpencodeSession(pool, storage, repoPath, startedAt, prompt)
	}
	if sessionErr != nil {
		if runErr != nil {
			return errors.Join(runErr, sessionErr)
		}
		return sessionErr
	}

	status := workspace.OpencodeSessionCompleted
	if exitCode != 0 {
		status = workspace.OpencodeSessionFailed
	}
	if _, err := pool.CompleteOpencodeSession(repoPath, session.ID, status, completedAt, &exitCode, int(completedAt.Sub(session.StartedAt).Seconds())); err != nil {
		return err
	}

	if runErr != nil {
		return runErr
	}
	if exitCode != 0 {
		return exitError{code: exitCode}
	}
	return nil
}

func resolveOpencodePrompt(args []string, reader io.Reader) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read prompt from stdin: %w", err)
	}

	prompt := strings.TrimSuffix(string(data), "\n")
	prompt = strings.TrimSuffix(prompt, "\r")
	return prompt, nil
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

func ensureOpencodeSession(pool *workspace.Pool, storage internalopencode.Storage, repoPath string, startedAt time.Time, prompt string) (workspace.OpencodeSession, error) {
	metadata, err := storage.FindSessionForRunWithRetry(repoPath, startedAt, prompt, opencodeSessionLookupTimeout)
	if err != nil {
		return workspace.OpencodeSession{}, err
	}

	sessionStartedAt := startedAt
	if !metadata.CreatedAt.IsZero() {
		sessionStartedAt = metadata.CreatedAt
	}

	if existing, err := pool.FindOpencodeSession(repoPath, metadata.ID); err == nil {
		if existing.Status == workspace.OpencodeSessionActive {
			return existing, nil
		}
	} else if !errors.Is(err, workspace.ErrOpencodeSessionNotFound) {
		return workspace.OpencodeSession{}, err
	}

	return pool.CreateOpencodeSession(repoPath, metadata.ID, prompt, sessionStartedAt)
}
