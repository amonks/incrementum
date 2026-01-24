package opencode

import (
	"context"
	"fmt"
	"io"
	"time"

	internalopencode "github.com/amonks/incrementum/internal/opencode"
)

// Logs returns a snapshot of opencode session logs.
func (s *Store) Logs(repoPath, sessionID string) (string, error) {
	session, err := s.FindSession(repoPath, sessionID)
	if err != nil {
		return "", err
	}
	return s.LogSnapshot(session.ID)
}

// LogSnapshot returns the session transcript as a single string.
func (s *Store) LogSnapshot(sessionID string) (string, error) {
	return s.storage.SessionLogText(sessionID)
}

// ProseLogs returns a snapshot of session logs without tool output.
func (s *Store) ProseLogs(repoPath, sessionID string) (string, error) {
	session, err := s.FindSession(repoPath, sessionID)
	if err != nil {
		return "", err
	}
	return s.ProseLogSnapshot(session.ID)
}

// ProseLogSnapshot returns the session transcript without tool output.
func (s *Store) ProseLogSnapshot(sessionID string) (string, error) {
	return s.storage.SessionProseLogText(sessionID)
}

// Tail streams opencode session logs until the context is cancelled.
func (s *Store) Tail(ctx context.Context, repoPath, sessionID string, writer io.Writer, interval time.Duration) error {
	session, err := s.FindSession(repoPath, sessionID)
	if err != nil {
		return err
	}
	return tailLog(ctx, s.storage, session.ID, writer, interval)
}

func tailLog(ctx context.Context, storage interface {
	SessionLogEntries(string) ([]internalopencode.LogEntry, error)
}, sessionID string, writer io.Writer, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}

	seen := make(map[string]struct{})
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		entries, err := storage.SessionLogEntries(sessionID)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if _, ok := seen[entry.ID]; ok {
				continue
			}
			if _, writeErr := writer.Write([]byte(entry.Text)); writeErr != nil {
				return fmt.Errorf("write opencode log: %w", writeErr)
			}
			seen[entry.ID] = struct{}{}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}
