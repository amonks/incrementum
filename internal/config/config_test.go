package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/internal/config"
)

func TestLoad_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if cfg.Workspace.OnCreate != "" {
		t.Error("expected empty OnCreate")
	}

	if cfg.Workspace.OnAcquire != "" {
		t.Error("expected empty OnAcquire")
	}
}

func TestLoad_Full(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = """
npm install
go mod download
"""
on-acquire = "npm install"
`

	if err := os.WriteFile(filepath.Join(tmpDir, ".incr.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate == "" {
		t.Error("expected non-empty OnCreate")
	}

	if cfg.Workspace.OnAcquire != "npm install" {
		t.Errorf("OnAcquire = %q, expected %q", cfg.Workspace.OnAcquire, "npm install")
	}
}

func TestLoad_WithShebang(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = """
#!/usr/bin/env python3
print("hello from python")
"""
`

	if err := os.WriteFile(filepath.Join(tmpDir, ".incr.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate == "" {
		t.Error("expected non-empty OnCreate")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `this is not valid toml [`

	if err := os.WriteFile(filepath.Join(tmpDir, ".incr.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestRunScript_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty script should be a no-op
	if err := config.RunScript(tmpDir, ""); err != nil {
		t.Errorf("unexpected error for empty script: %v", err)
	}

	if err := config.RunScript(tmpDir, "   "); err != nil {
		t.Errorf("unexpected error for whitespace script: %v", err)
	}
}

func TestRunScript_SimpleBash(t *testing.T) {
	tmpDir := t.TempDir()

	script := `touch created.txt`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "created.txt")); os.IsNotExist(err) {
		t.Error("script did not create file")
	}
}

func TestRunScript_MultipleBashCommands(t *testing.T) {
	tmpDir := t.TempDir()

	script := `
touch file1.txt
touch file2.txt
echo "done"
`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("script did not create file1.txt")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "file2.txt")); os.IsNotExist(err) {
		t.Error("script did not create file2.txt")
	}
}

func TestRunScript_ExplicitBashShebang(t *testing.T) {
	tmpDir := t.TempDir()

	script := `#!/bin/bash
touch from_bash.txt
`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "from_bash.txt")); os.IsNotExist(err) {
		t.Error("script did not create file")
	}
}

func TestRunScript_ShebangWithArgs(t *testing.T) {
	tmpDir := t.TempDir()

	// Use bash -e to exit on first error
	script := `#!/bin/bash -e
touch success.txt
`

	if err := config.RunScript(tmpDir, script); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "success.txt")); os.IsNotExist(err) {
		t.Error("script did not create file")
	}
}

func TestRunScript_FailingScript(t *testing.T) {
	tmpDir := t.TempDir()

	script := `exit 1`

	if err := config.RunScript(tmpDir, script); err == nil {
		t.Error("expected error for failing script")
	}
}
