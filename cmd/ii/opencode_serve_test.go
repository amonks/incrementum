package main

import (
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestOpencodeDaemonLogPath(t *testing.T) {
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
	logPath, err := opencodeDaemonLogPath(pool, repoPath)
	if err != nil {
		t.Fatalf("get log path: %v", err)
	}

	expected := filepath.Join(home, ".local", "share", "incrementum", "opencode", "tmp-my-repo", "daemon.log")
	if logPath != expected {
		t.Fatalf("expected log path %q, got %q", expected, logPath)
	}
}
