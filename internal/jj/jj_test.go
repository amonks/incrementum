package jj_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/internal/jj"
)

func TestWorkspaceRoot(t *testing.T) {
	// Create a temporary jj repo
	tmpDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	client := jj.New()

	// Init a jj repo
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	// Test WorkspaceRoot
	root, err := client.WorkspaceRoot(tmpDir)
	if err != nil {
		t.Fatalf("failed to get workspace root: %v", err)
	}

	if root != tmpDir {
		t.Errorf("expected root %q, got %q", tmpDir, root)
	}
}

func TestWorkspaceRoot_NotARepo(t *testing.T) {
	tmpDir := t.TempDir()
	client := jj.New()

	_, err := client.WorkspaceRoot(tmpDir)
	if err == nil {
		t.Error("expected error for non-repo directory")
	}
}

func TestWorkspaceAdd(t *testing.T) {
	tmpDir := t.TempDir()
	client := jj.New()

	// Init a jj repo
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	// Add a workspace - create parent directory first
	wsPath := filepath.Join(tmpDir, "workspaces", "ws-001")
	if err := os.MkdirAll(filepath.Dir(wsPath), 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	if err := client.WorkspaceAdd(tmpDir, "ws-001", wsPath); err != nil {
		t.Fatalf("failed to add workspace: %v", err)
	}

	// Verify the workspace exists
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Error("workspace directory was not created")
	}
}

func TestWorkspaceList(t *testing.T) {
	tmpDir := t.TempDir()
	client := jj.New()

	// Init a jj repo
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	// List workspaces (should have default workspace)
	workspaces, err := client.WorkspaceList(tmpDir)
	if err != nil {
		t.Fatalf("failed to list workspaces: %v", err)
	}

	if len(workspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(workspaces))
	}

	// Add a workspace and list again
	wsPath := filepath.Join(tmpDir, "workspaces", "ws-001")
	if err := os.MkdirAll(filepath.Dir(wsPath), 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	if err := client.WorkspaceAdd(tmpDir, "ws-001", wsPath); err != nil {
		t.Fatalf("failed to add workspace: %v", err)
	}

	workspaces, err = client.WorkspaceList(tmpDir)
	if err != nil {
		t.Fatalf("failed to list workspaces: %v", err)
	}

	if len(workspaces) != 2 {
		t.Errorf("expected 2 workspaces, got %d", len(workspaces))
	}
}

func TestEdit(t *testing.T) {
	tmpDir := t.TempDir()
	client := jj.New()

	// Init a jj repo
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	// Edit to a specific revision (@ should always work)
	if err := client.Edit(tmpDir, "@"); err != nil {
		t.Fatalf("failed to edit: %v", err)
	}
}

func TestCurrentChangeID(t *testing.T) {
	tmpDir := t.TempDir()
	client := jj.New()

	// Init a jj repo
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	// Get current change ID
	changeID, err := client.CurrentChangeID(tmpDir)
	if err != nil {
		t.Fatalf("failed to get current change ID: %v", err)
	}

	if changeID == "" {
		t.Error("expected non-empty change ID")
	}
}
