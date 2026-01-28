package opencode

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultRootUsesXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join("/tmp", "xdg-data"))
	t.Setenv("HOME", filepath.Join("/tmp", "home"))

	root, err := DefaultRoot()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "xdg-data", "opencode")
	if root != expected {
		t.Fatalf("expected %s, got %s", expected, root)
	}
}

func TestDefaultRootUsesHomeDirWhenXDGUnset(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", filepath.Join("/tmp", "home"))

	root, err := DefaultRoot()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "home", ".local", "share", "opencode")
	if root != expected {
		t.Fatalf("expected %s, got %s", expected, root)
	}
}

func TestSelectSessionNotFoundError(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)
	store := Storage{Root: root}

	_, err := store.selectSession(nil, repoPath, startedAt, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errSessionNotFound) {
		t.Fatalf("expected errSessionNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "repo="+repoPath) {
		t.Fatalf("expected repo path in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "started="+formatTimeLabel(startedAt)) {
		t.Fatalf("expected started label in error, got %q", err.Error())
	}
	cutoff := startedAt.Add(-5 * time.Second)
	if !strings.Contains(err.Error(), "cutoff="+formatTimeLabel(cutoff)) {
		t.Fatalf("expected cutoff label in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "total=0") {
		t.Fatalf("expected total count in error, got %q", err.Error())
	}
	storagePath := filepath.Join(root, "storage")
	if !strings.Contains(err.Error(), "storage="+storagePath) {
		t.Fatalf("expected storage path in error, got %q", err.Error())
	}
}

func TestSessionLogTextFormatsToolOutput(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	sessionID := "ses_format"
	messageID := "msg_tool"

	writeMessageRecord(t, root, sessionID, messageID, "assistant", 1000)
	writePartRecord(t, root, messageID, "prt_text", map[string]any{
		"type": "text",
		"text": "Hello\n",
		"time": map[string]any{
			"start": int64(1000),
		},
	})
	writePartRecord(t, root, messageID, "prt_tool", map[string]any{
		"type": "tool",
		"state": map[string]any{
			"output": map[string]any{
				"stdout": "Tool output\n",
				"stderr": "Tool error\n",
			},
		},
		"time": map[string]any{
			"start": int64(2000),
		},
	})

	snapshot, err := store.SessionLogText(sessionID)
	if err != nil {
		t.Fatalf("read session log: %v", err)
	}

	expected := "Hello\nStdout:\n    Tool output\nStderr:\n    Tool error\n"
	if snapshot != expected {
		t.Fatalf("expected log %q, got %q", expected, snapshot)
	}
}

func TestSessionProseLogTextFiltersToolOutput(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	sessionID := "ses_prose"
	userMessageID := "msg_user"
	assistantMessageID := "msg_assistant"

	writeMessageRecord(t, root, sessionID, userMessageID, "user", 1000)
	writeMessageRecord(t, root, sessionID, assistantMessageID, "assistant", 2000)
	writePartRecord(t, root, userMessageID, "prt_user_text", map[string]any{
		"type": "text",
		"text": "Hello\n",
		"time": map[string]any{
			"start": int64(1000),
		},
	})
	writePartRecord(t, root, assistantMessageID, "prt_tool", map[string]any{
		"type": "tool",
		"state": map[string]any{
			"output": map[string]any{
				"stdout": "Noise\n",
			},
		},
		"time": map[string]any{
			"start": int64(1500),
		},
	})
	writePartRecord(t, root, assistantMessageID, "prt_text", map[string]any{
		"type": "text",
		"text": "Goodbye\n",
		"time": map[string]any{
			"start": int64(2500),
		},
	})

	prose, err := store.SessionProseLogText(sessionID)
	if err != nil {
		t.Fatalf("read prose log: %v", err)
	}

	expected := "Hello\nGoodbye\n"
	if prose != expected {
		t.Fatalf("expected prose log %q, got %q", expected, prose)
	}
}

func TestListSessionsSupportsSecondTimestamps(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	projectID := "proj_seconds"
	created := int64(1700000000)
	createdAt := time.Unix(created, 0)

	sessionDir := filepath.Join(root, "storage", "session", projectID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("create session dir: %v", err)
	}
	writeJSON(t, filepath.Join(sessionDir, "ses_seconds.json"), map[string]any{
		"id":        "ses_seconds",
		"projectID": projectID,
		"directory": filepath.Join(root, "repo"),
		"time": map[string]any{
			"created": created,
		},
	})

	sessions, err := store.listSessions(projectID)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if !sessions[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("expected created time %s, got %s", createdAt, sessions[0].CreatedAt)
	}
}

func TestSelectSessionUsesPromptMatch(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	repoPath := filepath.Join(root, "repo")
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	entries := []SessionMetadata{
		{
			ID:        "ses_first",
			Directory: repoPath,
			CreatedAt: startedAt.Add(1 * time.Second),
		},
		{
			ID:        "ses_second",
			Directory: repoPath,
			CreatedAt: startedAt.Add(2 * time.Second),
		},
	}

	writeMessageRecord(t, root, "ses_first", "msg_first", "user", 1000)
	writePartRecord(t, root, "msg_first", "prt_first", map[string]any{
		"type": "text",
		"text": "No match here\n",
		"time": map[string]any{
			"start": int64(1000),
		},
	})
	writeMessageRecord(t, root, "ses_second", "msg_second", "user", 2000)
	writePartRecord(t, root, "msg_second", "prt_second", map[string]any{
		"type": "text",
		"text": "Please Match me\n",
		"time": map[string]any{
			"start": int64(2000),
		},
	})

	selected, err := store.selectSession(entries, repoPath, startedAt, "Match me")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != "ses_second" {
		t.Fatalf("expected prompt match session, got %q", selected.ID)
	}
}

func TestSelectSessionUsesUpdatedAtWhenCreatedIsStale(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	repoPath := filepath.Join(root, "repo")
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	entries := []SessionMetadata{
		{
			ID:        "ses_recent",
			Directory: repoPath,
			CreatedAt: startedAt.Add(-10 * time.Second),
			UpdatedAt: startedAt.Add(2 * time.Second),
		},
	}

	selected, err := store.selectSession(entries, repoPath, startedAt, "")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != "ses_recent" {
		t.Fatalf("expected updated session, got %q", selected.ID)
	}
}

func TestSelectSessionFallsBackToPromptMatchWhenNoRecentSessions(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	repoPath := filepath.Join(root, "repo")
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	entries := []SessionMetadata{
		{
			ID:        "ses_old",
			Directory: repoPath,
			CreatedAt: startedAt.Add(-15 * time.Minute),
		},
	}

	writeMessageRecord(t, root, "ses_old", "msg_old", "user", 1000)
	writePartRecord(t, root, "msg_old", "prt_old", map[string]any{
		"type": "text",
		"text": "Please Match me\n",
		"time": map[string]any{
			"start": int64(1000),
		},
	})

	selected, err := store.selectSession(entries, repoPath, startedAt, "Match me")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != "ses_old" {
		t.Fatalf("expected prompt match session, got %q", selected.ID)
	}
}

func TestSelectSessionPromptMatchFallbackStaysWithinRepo(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	repoPath := filepath.Join(root, "repo")
	otherRepo := filepath.Join(root, "other")
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	entries := []SessionMetadata{
		{
			ID:        "ses_repo",
			Directory: repoPath,
			CreatedAt: startedAt.Add(-10 * time.Minute),
		},
		{
			ID:        "ses_other",
			Directory: otherRepo,
			CreatedAt: startedAt.Add(-9 * time.Minute),
		},
	}

	writeMessageRecord(t, root, "ses_other", "msg_other", "user", 1000)
	writePartRecord(t, root, "msg_other", "prt_other", map[string]any{
		"type": "text",
		"text": "Please Match me\n",
		"time": map[string]any{
			"start": int64(1000),
		},
	})

	selected, err := store.selectSession(entries, repoPath, startedAt, "Match me")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != "ses_repo" {
		t.Fatalf("expected repo session, got %q", selected.ID)
	}
}

func TestSelectSessionFallsBackToLatestSessionWhenNoRecentSessions(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	repoPath := filepath.Join(root, "repo")
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	entries := []SessionMetadata{
		{
			ID:        "ses_old",
			Directory: repoPath,
			CreatedAt: startedAt.Add(-20 * time.Minute),
			UpdatedAt: startedAt.Add(-19 * time.Minute),
		},
		{
			ID:        "ses_latest",
			Directory: repoPath,
			CreatedAt: startedAt.Add(-18 * time.Minute),
			UpdatedAt: startedAt.Add(-10 * time.Minute),
		},
	}

	selected, err := store.selectSession(entries, repoPath, startedAt, "")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != "ses_latest" {
		t.Fatalf("expected latest session, got %q", selected.ID)
	}
}

func TestSelectSessionLatestFallbackStaysWithinRepo(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	repoPath := filepath.Join(root, "repo")
	otherRepo := filepath.Join(root, "other")
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	entries := []SessionMetadata{
		{
			ID:        "ses_old",
			Directory: repoPath,
			CreatedAt: startedAt.Add(-30 * time.Minute),
			UpdatedAt: startedAt.Add(-20 * time.Minute),
		},
		{
			ID:        "ses_other",
			Directory: otherRepo,
			CreatedAt: startedAt.Add(-10 * time.Minute),
			UpdatedAt: startedAt.Add(-5 * time.Minute),
		},
	}

	selected, err := store.selectSession(entries, repoPath, startedAt, "")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != "ses_old" {
		t.Fatalf("expected repo latest session, got %q", selected.ID)
	}
}

func TestSelectSessionMatchesSymlinkedRepoPath(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}
	actualRepo := filepath.Join(root, "repo")
	if err := os.MkdirAll(actualRepo, 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
	}
	linkRepo := filepath.Join(root, "repo-link")
	if err := os.Symlink(actualRepo, linkRepo); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	startedAt := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	entries := []SessionMetadata{
		{
			ID:        "ses_link",
			Directory: actualRepo,
			CreatedAt: startedAt.Add(1 * time.Second),
		},
	}

	selected, err := store.selectSession(entries, linkRepo, startedAt, "")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != "ses_link" {
		t.Fatalf("expected symlink session, got %q", selected.ID)
	}
}

func writeMessageRecord(t *testing.T, root, sessionID, messageID, role string, createdAt int64) {
	t.Helper()

	messageDir := filepath.Join(root, "storage", "message", sessionID)
	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		t.Fatalf("create message dir: %v", err)
	}
	writeJSON(t, filepath.Join(messageDir, messageID+".json"), map[string]any{
		"id":        messageID,
		"sessionID": sessionID,
		"role":      role,
		"time": map[string]any{
			"created": createdAt,
		},
	})
}

func writePartRecord(t *testing.T, root, messageID, partID string, record map[string]any) {
	t.Helper()

	partDir := filepath.Join(root, "storage", "part", messageID)
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatalf("create part dir: %v", err)
	}
	data := map[string]any{
		"id":        partID,
		"messageID": messageID,
	}
	for key, value := range record {
		data[key] = value
	}
	writeJSON(t, filepath.Join(partDir, partID+".json"), data)
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
