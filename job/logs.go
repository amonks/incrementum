package job

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amonks/incrementum/opencode"
)

type logEntry struct {
	Purpose string
	Session opencode.OpencodeSession
	Log     string
}

// LogSnapshot returns the full opencode logs for the job's sessions.
func LogSnapshot(store *opencode.Store, repoPath string, item Job) (string, error) {
	if store == nil {
		return "", fmt.Errorf("opencode store is required")
	}
	if len(item.OpencodeSessions) == 0 {
		return "", nil
	}

	entries := make([]logEntry, 0, len(item.OpencodeSessions))
	for _, session := range item.OpencodeSessions {
		entry, err := logEntryForSession(store, repoPath, session)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Session.StartedAt.Equal(entries[j].Session.StartedAt) {
			return entries[i].Session.ID < entries[j].Session.ID
		}
		return entries[i].Session.StartedAt.Before(entries[j].Session.StartedAt)
	})

	var builder strings.Builder
	for i, entry := range entries {
		if i > 0 {
			builder.WriteString("\n")
		}
		fmt.Fprintf(&builder, "==> %s (%s)\n", entry.Purpose, entry.Session.ID)
		builder.WriteString(entry.Log)
		if !strings.HasSuffix(entry.Log, "\n") {
			builder.WriteString("\n")
		}
	}

	return builder.String(), nil
}

func logEntryForSession(store *opencode.Store, repoPath string, session OpencodeSession) (logEntry, error) {
	opencodeSession, err := store.FindSession(repoPath, session.ID)
	if err != nil {
		return logEntry{}, err
	}
	logSnapshot, err := store.LogSnapshot(opencodeSession.ID)
	if err != nil {
		return logEntry{}, err
	}

	return logEntry{
		Purpose: session.Purpose,
		Session: opencodeSession,
		Log:     logSnapshot,
	}, nil
}
