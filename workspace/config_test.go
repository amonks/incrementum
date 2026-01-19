package workspace_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestLoadConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := workspace.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty config if file doesn't exist
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if len(cfg.Workspace.OnCreate) != 0 {
		t.Error("expected empty OnCreate")
	}

	if len(cfg.Workspace.OnAcquire) != 0 {
		t.Error("expected empty OnAcquire")
	}
}

func TestLoadConfig_Full(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = ["npm install", "go mod download"]
on-acquire = ["npm install"]
`

	if err := os.WriteFile(filepath.Join(tmpDir, ".incr.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := workspace.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	expectedOnCreate := []string{"npm install", "go mod download"}
	if !reflect.DeepEqual(cfg.Workspace.OnCreate, expectedOnCreate) {
		t.Errorf("OnCreate = %v, expected %v", cfg.Workspace.OnCreate, expectedOnCreate)
	}

	expectedOnAcquire := []string{"npm install"}
	if !reflect.DeepEqual(cfg.Workspace.OnAcquire, expectedOnAcquire) {
		t.Errorf("OnAcquire = %v, expected %v", cfg.Workspace.OnAcquire, expectedOnAcquire)
	}
}

func TestLoadConfig_OnlyOnCreate(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = ["make setup"]
`

	if err := os.WriteFile(filepath.Join(tmpDir, ".incr.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := workspace.LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Workspace.OnCreate) != 1 {
		t.Errorf("expected 1 OnCreate command, got %d", len(cfg.Workspace.OnCreate))
	}

	if len(cfg.Workspace.OnAcquire) != 0 {
		t.Error("expected empty OnAcquire")
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `this is not valid toml [`

	if err := os.WriteFile(filepath.Join(tmpDir, ".incr.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := workspace.LoadConfig(tmpDir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}
