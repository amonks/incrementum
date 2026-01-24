package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestGetOpencodeRepoPathUsesRepoRootForWorkspace(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create home: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	t.Setenv("HOME", home)

	repoPath := setupTestRepo(t)

	pool, err := workspace.Open()
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{Purpose: "opencode test"})
	if err != nil {
		t.Fatalf("acquire workspace: %v", err)
	}

	withCwd(t, wsPath, func() {
		resolved, err := getOpencodeRepoPath()
		if err != nil {
			t.Fatalf("get opencode repo path: %v", err)
		}
		if resolved != repoPath {
			t.Fatalf("expected repo path %q, got %q", repoPath, resolved)
		}
	})
}

func TestGetOpencodeRepoPathFallsBackToWorkingDir(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create home: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	t.Setenv("HOME", home)

	cwd := filepath.Join(root, "cwd")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("create cwd: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}

	withCwd(t, cwd, func() {
		resolved, err := getOpencodeRepoPath()
		if err != nil {
			t.Fatalf("get opencode repo path: %v", err)
		}
		if resolved != cwd {
			t.Fatalf("expected repo path %q, got %q", cwd, resolved)
		}
	})
}
