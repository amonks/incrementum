package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func TestOpencodeSessionLogPath(t *testing.T) {
	pool, err := workspace.OpenWithOptions(workspace.Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	logPath := filepath.Join(t.TempDir(), "session.log")
	startedAt := time.Now().UTC()

	session, err := pool.CreateOpencodeSession(repoPath, "Run tests", logPath, startedAt)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	found, err := opencodeSessionLogPath(pool, repoPath, session.ID)
	if err != nil {
		t.Fatalf("get log path: %v", err)
	}
	if found != logPath {
		t.Fatalf("expected log path %q, got %q", logPath, found)
	}
}

func TestOpencodeLogSnapshot(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "session.log")
	content := "line one\nline two\n"

	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	snapshot, err := opencodeLogSnapshot(logPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snapshot != content {
		t.Fatalf("expected snapshot %q, got %q", content, snapshot)
	}
}
