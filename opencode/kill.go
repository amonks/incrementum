package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Kill terminates an opencode session and updates state.
func (s *Store) Kill(repoPath, sessionID string) (OpencodeSession, error) {
	stored, err := s.FindSession(repoPath, sessionID)
	if err != nil {
		return OpencodeSession{}, err
	}
	resolvedID := stored.ID
	if resolvedID == "" {
		resolvedID = sessionID
	}

	metadata, err := sessionKill(resolvedID)
	if err != nil {
		return OpencodeSession{}, err
	}

	status := OpencodeSessionKilled
	completedAt := time.Now()
	duration := metadata.DurationSeconds
	if duration == 0 && !stored.StartedAt.IsZero() {
		duration = int(completedAt.Sub(stored.StartedAt).Seconds())
	}

	updated, err := s.CompleteSession(repoPath, resolvedID, status, completedAt, metadata.ExitCode, duration)
	if err != nil {
		if errors.Is(err, ErrOpencodeSessionNotActive) {
			return stored, nil
		}
		return OpencodeSession{}, err
	}

	return updated, nil
}

type sessionMetadata struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}

type sessionList struct {
	Sessions []sessionMetadata `json:"sessions"`
}

func sessionKill(sessionID string) (sessionMetadata, error) {
	cmd := exec.Command("opencode", "session", "kill", sessionID, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return sessionMetadata{}, fmt.Errorf("opencode session kill failed: %s", stderr)
			}
		}
		return sessionMetadata{}, fmt.Errorf("opencode session kill failed: %w", err)
	}

	session, err := decodeSessionMetadata(output)
	if err != nil {
		return sessionMetadata{}, err
	}
	if session.ID == "" {
		return sessionMetadata{}, fmt.Errorf("opencode session kill response missing session id")
	}
	return session, nil
}

func decodeSessionMetadata(data []byte) (sessionMetadata, error) {
	var session sessionMetadata
	if err := json.Unmarshal(data, &session); err == nil && session.ID != "" {
		return session, nil
	}

	var envelope struct {
		Session sessionMetadata `json:"session"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Session.ID != "" {
		return envelope.Session, nil
	}

	sessions, err := decodeSessionList(data)
	if err == nil && len(sessions) > 0 {
		return sessions[0], nil
	}
	if err == nil {
		return sessionMetadata{}, fmt.Errorf("decode opencode session metadata: no session in response")
	}
	return sessionMetadata{}, fmt.Errorf("decode opencode session metadata: %w", err)
}

func decodeSessionList(data []byte) ([]sessionMetadata, error) {
	var sessions []sessionMetadata
	if err := json.Unmarshal(data, &sessions); err == nil {
		return sessions, nil
	}

	var envelope sessionList
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode opencode session list: %w", err)
	}
	return envelope.Sessions, nil
}
