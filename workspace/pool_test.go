package workspace_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	// Init a jj repo
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

func TestPool_Acquire_CreatesNewWorkspace(t *testing.T) {
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

	// Verify workspace path exists
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Error("workspace directory was not created")
	}

	// Verify it's a jj workspace
	if _, err := os.Stat(filepath.Join(wsPath, ".jj")); os.IsNotExist(err) {
		t.Error("workspace does not have .jj directory")
	}
}

func TestPool_Acquire_ReusesAvailableWorkspace(t *testing.T) {
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

	// Claim and release a workspace
	wsPath1, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to claim workspace: %v", err)
	}

	if err := pool.Release(wsPath1); err != nil {
		t.Fatalf("failed to release workspace: %v", err)
	}

	// Claim again - should reuse the same workspace
	wsPath2, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to claim workspace second time: %v", err)
	}

	if wsPath1 != wsPath2 {
		t.Errorf("expected to reuse workspace %q, got %q", wsPath1, wsPath2)
	}
}

func TestPool_Acquire_CreatesMultipleWorkspaces(t *testing.T) {
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

	// Claim two workspaces without releasing
	wsPath1, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to claim workspace 1: %v", err)
	}

	wsPath2, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to claim workspace 2: %v", err)
	}

	if wsPath1 == wsPath2 {
		t.Error("expected different workspaces, got same path")
	}

	// Both should contain ws- prefix and be numbered
	if !strings.Contains(wsPath1, "ws-") {
		t.Errorf("expected ws- prefix in %q", wsPath1)
	}
	if !strings.Contains(wsPath2, "ws-") {
		t.Errorf("expected ws- prefix in %q", wsPath2)
	}
}

func TestPool_Release(t *testing.T) {
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
		t.Fatalf("failed to claim workspace: %v", err)
	}

	if err := pool.Release(wsPath); err != nil {
		t.Fatalf("failed to release workspace: %v", err)
	}
}

func TestPool_Renew(t *testing.T) {
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
		t.Fatalf("failed to claim workspace: %v", err)
	}

	// Heartbeat should extend the lease
	if err := pool.Renew(wsPath); err != nil {
		t.Fatalf("failed to heartbeat: %v", err)
	}
}

func TestPool_List(t *testing.T) {
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

	// Initially empty
	list, err := pool.List(repoPath)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(list))
	}

	// Claim one
	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to claim: %v", err)
	}

	list, err = pool.List(repoPath)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(list))
	}

	if list[0].Path != wsPath {
		t.Errorf("expected path %q, got %q", wsPath, list[0].Path)
	}

	if list[0].Status != workspace.StatusAcquired {
		t.Errorf("expected status claimed, got %s", list[0].Status)
	}

	// Should have a current change ID
	if list[0].CurrentChangeID == "" {
		t.Error("expected non-empty current change ID")
	}
}

func TestPool_ExpiresStaleLeases(t *testing.T) {
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

	// Claim with very short TTL
	wsPath1, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Millisecond})
	if err != nil {
		t.Fatalf("failed to claim: %v", err)
	}

	// Wait for it to expire
	time.Sleep(10 * time.Millisecond)

	// Next claim should reuse the expired workspace
	wsPath2, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to claim after expiry: %v", err)
	}

	if wsPath1 != wsPath2 {
		t.Errorf("expected expired workspace to be reused, got %q and %q", wsPath1, wsPath2)
	}
}

func TestPool_DefaultOptions(t *testing.T) {
	// Just verify Open() doesn't error
	pool, err := workspace.Open()
	if err != nil {
		t.Fatalf("failed to open pool with defaults: %v", err)
	}
	if pool == nil {
		t.Error("expected non-nil pool")
	}
}

func TestRepoRoot(t *testing.T) {
	repoPath := setupTestRepo(t)

	root, err := workspace.RepoRoot(repoPath)
	if err != nil {
		t.Fatalf("failed to get repo root: %v", err)
	}

	if root != repoPath {
		t.Errorf("expected %q, got %q", repoPath, root)
	}
}

func TestRepoRoot_NotARepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := workspace.RepoRoot(tmpDir)
	if err == nil {
		t.Error("expected error for non-repo directory")
	}
}

func TestRepoRootFromPath_Workspace(t *testing.T) {
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

	root, err := workspace.RepoRootFromPathWithOptions(wsPath, workspace.Options{
		StateDir:      stateDir,
		WorkspacesDir: workspacesDir,
	})
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	if root != repoPath {
		t.Fatalf("expected repo path %q, got %q", repoPath, root)
	}
}

func TestRepoRootFromPath_Repo(t *testing.T) {
	repoPath := setupTestRepo(t)

	root, err := workspace.RepoRootFromPathWithOptions(repoPath, workspace.Options{
		StateDir:      "",
		WorkspacesDir: "",
	})
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	if root != repoPath {
		t.Fatalf("expected repo path %q, got %q", repoPath, root)
	}
}

func TestRepoRootFromPath_NotARepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := workspace.RepoRootFromPath(tmpDir)
	if err == nil {
		t.Fatal("expected error for non-repo directory")
	}
}

func TestPool_DestroyAll(t *testing.T) {
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

	// Acquire two workspaces
	wsPath1, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to acquire workspace 1: %v", err)
	}

	wsPath2, err := pool.Acquire(repoPath, workspace.AcquireOptions{TTL: time.Hour})
	if err != nil {
		t.Fatalf("failed to acquire workspace 2: %v", err)
	}

	// Verify workspaces exist
	if _, err := os.Stat(wsPath1); os.IsNotExist(err) {
		t.Fatalf("workspace 1 does not exist: %s", wsPath1)
	}
	if _, err := os.Stat(wsPath2); os.IsNotExist(err) {
		t.Fatalf("workspace 2 does not exist: %s", wsPath2)
	}

	// Destroy all
	if err := pool.DestroyAll(repoPath); err != nil {
		t.Fatalf("failed to destroy all: %v", err)
	}

	// Verify workspaces are gone
	if _, err := os.Stat(wsPath1); !os.IsNotExist(err) {
		t.Error("workspace 1 should have been deleted")
	}
	if _, err := os.Stat(wsPath2); !os.IsNotExist(err) {
		t.Error("workspace 2 should have been deleted")
	}

	// List should return empty
	list, err := pool.List(repoPath)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 workspaces after destroy-all, got %d", len(list))
	}
}

func TestPool_DestroyAll_NoWorkspaces(t *testing.T) {
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

	// Destroy all when there are no workspaces should not error
	if err := pool.DestroyAll(repoPath); err != nil {
		t.Fatalf("destroy-all with no workspaces should not error: %v", err)
	}
}

func TestPool_WorkspaceNameForPath(t *testing.T) {
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

	name, err := pool.WorkspaceNameForPath(wsPath)
	if err != nil {
		t.Fatalf("failed to resolve workspace name: %v", err)
	}
	if name == "" {
		t.Fatal("expected workspace name")
	}
}

func TestPool_WorkspaceNameForPath_NotInWorkspace(t *testing.T) {
	pool, err := workspace.Open()
	if err != nil {
		t.Fatalf("failed to open pool: %v", err)
	}

	_, err = pool.WorkspaceNameForPath(t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-workspace directory")
	}
	if !errors.Is(err, workspace.ErrWorkspaceRootNotFound) {
		t.Fatalf("expected workspace root not found error, got %v", err)
	}
}
