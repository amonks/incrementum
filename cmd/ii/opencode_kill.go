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

var opencodeKillCmd = &cobra.Command{
	Use:   "kill <session-id>",
	Short: "Terminate an opencode session",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpencodeKill,
}

func init() {
	opencodeCmd.AddCommand(opencodeKillCmd)
}

func runOpencodeKill(cmd *cobra.Command, args []string) error {
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

	metadata, err := opencodeSessionKill(resolvedID)
	if err != nil {
		return err
	}

	status := resolveOpencodeKillStatus(metadata)

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

func opencodeSessionKill(sessionID string) (opencodeSessionMetadata, error) {
	cmd := exec.Command("opencode", "session", "kill", sessionID, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return opencodeSessionMetadata{}, fmt.Errorf("opencode session kill failed: %s", stderr)
			}
		}
		return opencodeSessionMetadata{}, fmt.Errorf("opencode session kill failed: %w", err)
	}

	session, err := decodeOpencodeSessionMetadata(output)
	if err != nil {
		return opencodeSessionMetadata{}, err
	}
	if session.ID == "" {
		return opencodeSessionMetadata{}, fmt.Errorf("opencode session kill response missing session id")
	}
	return session, nil
}

func resolveOpencodeKillStatus(metadata opencodeSessionMetadata) workspace.OpencodeSessionStatus {
	return workspace.OpencodeSessionKilled
}

func decodeOpencodeSessionMetadata(data []byte) (opencodeSessionMetadata, error) {
	var session opencodeSessionMetadata
	if err := json.Unmarshal(data, &session); err == nil && session.ID != "" {
		return session, nil
	}

	var envelope struct {
		Session opencodeSessionMetadata `json:"session"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Session.ID != "" {
		return envelope.Session, nil
	}

	sessions, err := decodeOpencodeSessionList(data)
	if err == nil && len(sessions) > 0 {
		return sessions[0], nil
	}
	if err == nil {
		return opencodeSessionMetadata{}, fmt.Errorf("decode opencode session metadata: no session in response")
	}
	return opencodeSessionMetadata{}, fmt.Errorf("decode opencode session metadata: %w", err)
}
