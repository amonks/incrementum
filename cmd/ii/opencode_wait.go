package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

var opencodeWaitCmd = &cobra.Command{
	Use:   "wait <session-id>",
	Short: "Wait for an opencode session",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpencodeWait,
}

var errOpencodeSessionMissing = errors.New("opencode session not found in list")

type opencodeSessionMetadata struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}

type opencodeSessionList struct {
	Sessions []opencodeSessionMetadata `json:"sessions"`
}

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit %d", e.code)
}

func (e exitError) ExitCode() int {
	return e.code
}

func (e exitError) Unwrap() error {
	return e.err
}

func init() {
	opencodeCmd.AddCommand(opencodeWaitCmd)
}

func runOpencodeWait(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getOpencodeRepoPath()
	if err != nil {
		return err
	}

	sessionID := args[0]
	stored, err := pool.FindOpencodeSession(repoPath, sessionID)
	if err != nil {
		return err
	}
	resolvedID := resolveOpencodeSessionID(sessionID, stored)
	if stored.Status != workspace.OpencodeSessionActive {
		return exitFromOpencodeSession(stored)
	}

	metadata, err := pollOpencodeSession(opencodeSessionListForWait, resolvedID, time.Second)
	if err != nil {
		return err
	}

	status, err := resolveOpencodeSessionStatus(metadata)
	if err != nil {
		return err
	}

	completedAt := time.Now()
	duration := metadata.DurationSeconds
	if duration == 0 && !stored.StartedAt.IsZero() {
		duration = int(completedAt.Sub(stored.StartedAt).Seconds())
	}

	updated, err := pool.CompleteOpencodeSession(repoPath, resolvedID, status, completedAt, metadata.ExitCode, duration)
	if err != nil {
		if errors.Is(err, workspace.ErrOpencodeSessionNotActive) {
			return exitFromOpencodeSession(stored)
		}
		return err
	}

	return exitFromOpencodeSession(updated)
}

func opencodeSessionListForWait() ([]opencodeSessionMetadata, error) {
	cmd := exec.Command("opencode", "session", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return nil, fmt.Errorf("opencode session list failed: %s", stderr)
			}
		}
		return nil, fmt.Errorf("opencode session list failed: %w", err)
	}

	sessions, err := decodeOpencodeSessionList(output)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func decodeOpencodeSessionList(data []byte) ([]opencodeSessionMetadata, error) {
	var sessions []opencodeSessionMetadata
	if err := json.Unmarshal(data, &sessions); err == nil {
		return sessions, nil
	}

	var envelope opencodeSessionList
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode opencode session list: %w", err)
	}
	return envelope.Sessions, nil
}

func pollOpencodeSession(list func() ([]opencodeSessionMetadata, error), sessionID string, interval time.Duration) (opencodeSessionMetadata, error) {
	for {
		sessions, err := list()
		if err != nil {
			return opencodeSessionMetadata{}, err
		}

		session, ok := findOpencodeSession(sessions, sessionID)
		if !ok {
			return opencodeSessionMetadata{}, errOpencodeSessionMissing
		}
		if session.Status != "active" || session.ExitCode != nil {
			return session, nil
		}
		if interval > 0 {
			time.Sleep(interval)
		}
	}
}

func findOpencodeSession(sessions []opencodeSessionMetadata, sessionID string) (opencodeSessionMetadata, bool) {
	for _, session := range sessions {
		if session.ID == sessionID {
			return session, true
		}
	}
	return opencodeSessionMetadata{}, false
}

func resolveOpencodeSessionStatus(metadata opencodeSessionMetadata) (workspace.OpencodeSessionStatus, error) {
	status := strings.ToLower(metadata.Status)
	switch status {
	case "completed":
		return workspace.OpencodeSessionCompleted, nil
	case "failed":
		return workspace.OpencodeSessionFailed, nil
	case "killed":
		return workspace.OpencodeSessionKilled, nil
	case "active":
		if metadata.ExitCode != nil {
			if *metadata.ExitCode == 0 {
				return workspace.OpencodeSessionCompleted, nil
			}
			return workspace.OpencodeSessionFailed, nil
		}
	}
	return "", fmt.Errorf("unsupported opencode session status: %s", metadata.Status)
}

func exitFromOpencodeSession(session workspace.OpencodeSession) error {
	if session.ExitCode != nil && *session.ExitCode != 0 {
		return exitError{code: *session.ExitCode}
	}
	return nil
}
