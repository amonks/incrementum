package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

var opencodeRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Start a new opencode session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runOpencodeRun,
}

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
	runCmd := exec.Command("opencode", "run", prompt)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	exitCode, runErr := runExitCode(runCmd)
	completedAt := time.Now()

	storage, err := opencodeStorage()
	if err != nil {
		return err
	}

	metadata, err := storage.FindSessionForRun(repoPath, startedAt, prompt)
	if err != nil {
		if runErr != nil {
			return errors.Join(runErr, err)
		}
		return err
	}

	sessionStartedAt := startedAt
	if !metadata.CreatedAt.IsZero() {
		sessionStartedAt = metadata.CreatedAt
	}

	session, err := pool.CreateOpencodeSession(repoPath, metadata.ID, prompt, sessionStartedAt)
	if err != nil {
		return err
	}

	status := workspace.OpencodeSessionCompleted
	if exitCode != 0 {
		status = workspace.OpencodeSessionFailed
	}
	if _, err := pool.CompleteOpencodeSession(repoPath, session.ID, status, completedAt, &exitCode, int(completedAt.Sub(sessionStartedAt).Seconds())); err != nil {
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
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
