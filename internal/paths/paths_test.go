package paths

import (
	"os"
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

func TestDefaultOpencodeEventsDirUsesHome(t *testing.T) {
	t.Setenv("HOME", filepath.Join("/tmp", "test-home"))

	dir, err := DefaultOpencodeEventsDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := filepath.Join("/tmp", "test-home", ".local", "share", "incrementum", "opencode", "events")
	if dir != expected {
		t.Fatalf("expected %s, got %s", expected, dir)
	}
}

func TestWorkingDirReturnsCurrentDir(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	workDir := t.TempDir()
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	resolved, err := WorkingDir()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resolved != workDir {
		t.Fatalf("expected %s, got %s", workDir, resolved)
	}
}
