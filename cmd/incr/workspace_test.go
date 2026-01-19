package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	if err := runJJ(tmpDir, "git", "init"); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	return tmpDir
}

func runJJ(dir string, args ...string) error {
	cmd := exec.Command("jj", args...)
	cmd.Dir = dir
	return cmd.Run()
}

func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	fn()
}

func TestResolveWorkspaceNameFromArgs(t *testing.T) {
	pool, err := workspace.Open()
	if err != nil {
		t.Fatalf("failed to open pool: %v", err)
	}

	name, err := resolveWorkspaceName([]string{"ws-123"}, pool)
	if err != nil {
		t.Fatalf("failed to resolve name: %v", err)
	}

	if name != "ws-123" {
		t.Fatalf("expected ws-123, got %q", name)
	}
}

func TestResolveWorkspaceNameFromCwd(t *testing.T) {
	repoPath := setupTestRepo(t)
	workspacesDir := t.TempDir()
	workspacesDir, _ = filepath.EvalSymlinks(workspacesDir)
	stateDir := t.TempDir()

	pool, err := workspace.OpenWithOptions(workspace.Options{
		StateDir:      stateDir,
		WorkspacesDir: workspacesDir,
	})
	if err != nil {
		t.Fatalf("failed to open pool: %v", err)
	}

	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to acquire workspace: %v", err)
	}

	expected := filepath.Base(wsPath)
	withCwd(t, wsPath, func() {
		name, err := resolveWorkspaceName(nil, pool)
		if err != nil {
			t.Fatalf("failed to resolve name: %v", err)
		}
		if name != expected {
			t.Fatalf("expected %q, got %q", expected, name)
		}
	})
}

func TestResolveWorkspaceNameFromCwd_NotInWorkspace(t *testing.T) {
	repoPath := setupTestRepo(t)
	workspacesDir := t.TempDir()
	workspacesDir, _ = filepath.EvalSymlinks(workspacesDir)
	stateDir := t.TempDir()

	pool, err := workspace.OpenWithOptions(workspace.Options{
		StateDir:      stateDir,
		WorkspacesDir: workspacesDir,
	})
	if err != nil {
		t.Fatalf("failed to open pool: %v", err)
	}

	withCwd(t, repoPath, func() {
		_, err := resolveWorkspaceName(nil, pool)
		if err == nil {
			t.Fatal("expected error when not in a workspace")
		}
		if !errors.Is(err, workspace.ErrRepoPathNotFound) {
			t.Fatalf("expected repo path not found error, got %v", err)
		}
	})
}
