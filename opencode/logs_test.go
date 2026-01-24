package opencode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogSnapshot(t *testing.T) {
	root := t.TempDir()
	store, err := OpenWithOptions(Options{
		StateDir:    t.TempDir(),
		StorageRoot: root,
		EventsDir:   filepath.Join(root, "events"),
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	sessionID := "ses_logtest"
	logDir := filepath.Join(root, "events")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("create events dir: %v", err)
	}
	logPath := filepath.Join(logDir, sessionID+".sse")
	if err := os.WriteFile(logPath, []byte("event: message\ndata: line one\ndata: line two\n\n"), 0o644); err != nil {
		t.Fatalf("write event log: %v", err)
	}

	snapshot, err := store.LogSnapshot(sessionID)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snapshot != "event: message\ndata: line one\ndata: line two\n\n" {
		t.Fatalf("expected snapshot to match event log, got %q", snapshot)
	}
}
