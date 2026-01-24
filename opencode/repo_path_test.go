package opencode

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestRepoPathForWorkingDirUsesRepoRootForWorkspace(t *testing.T) {
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
		resolved, err := RepoPathForWorkingDir()
		if err != nil {
			t.Fatalf("get opencode repo path: %v", err)
		}
		if resolved != repoPath {
			t.Fatalf("expected repo path %q, got %q", repoPath, resolved)
		}
	})
}

func TestRepoPathForWorkingDirFallsBackToWorkingDir(t *testing.T) {
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
		resolved, err := RepoPathForWorkingDir()
		if err != nil {
			t.Fatalf("get opencode repo path: %v", err)
		}
		if resolved != cwd {
			t.Fatalf("expected repo path %q, got %q", cwd, resolved)
		}
	})
}

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
