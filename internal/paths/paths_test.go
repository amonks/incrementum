package paths

import (
	"path/filepath"
	"testing"
)

func TestDefaultStateDirUsesHome(t *testing.T) {
	t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

	dir, err := DefaultStateDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "test-home", ".local", "state", "incrementum")
	if dir != expected {
		t.Fatalf("expected %s, got %s", expected, dir)
	}
}

func TestDefaultWorkspacesDirUsesHome(t *testing.T) {
	t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

	dir, err := DefaultWorkspacesDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "test-home", ".local", "share", "incrementum", "workspaces")
	if dir != expected {
		t.Fatalf("expected %s, got %s", expected, dir)
	}
}
