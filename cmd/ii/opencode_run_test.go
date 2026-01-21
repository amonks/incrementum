package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func TestOpencodeRunLogPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pool, err := workspace.OpenWithOptions(workspace.Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	prompt := "Run tests"
	startedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessionID, logPath, err := opencodeRunLogPath(pool, repoPath, prompt, startedAt)
	if err != nil {
		t.Fatalf("get run log path: %v", err)
	}

	expectedID := workspace.GenerateOpencodeSessionID(prompt, startedAt)
	expected := filepath.Join(home, ".local", "share", "incrementum", "opencode", "tmp-my-repo", expectedID+".log")
	if sessionID != expectedID {
		t.Fatalf("expected session id %q, got %q", expectedID, sessionID)
	}
	if logPath != expected {
		t.Fatalf("expected log path %q, got %q", expected, logPath)
	}
}
