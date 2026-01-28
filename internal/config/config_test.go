package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/internal/testsupport"
)

func TestLoad_NotFound(t *testing.T) {
	testsupport.SetupTestHome(t)
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
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = """
npm install
go mod download
"""
on-acquire = "npm install"
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
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
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[workspace]
on-create = """
#!/usr/bin/env python3
print("hello from python")
"""
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
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
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `this is not valid toml [`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := config.Load(tmpDir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestLoad_JobConfig(t *testing.T) {
	testsupport.SetupTestHome(t)
	tmpDir := t.TempDir()

	configContent := `
[job]
test-commands = ["go test ./...", "golangci-lint run"]
agent = "gpt-5.2-codex"
`

	if err := os.WriteFile(filepath.Join(tmpDir, "incrementum.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Job.TestCommands) != 2 {
		t.Fatalf("expected 2 test commands, got %d", len(cfg.Job.TestCommands))
	}

	if cfg.Job.TestCommands[0] != "go test ./..." {
		t.Fatalf("expected first test command %q, got %q", "go test ./...", cfg.Job.TestCommands[0])
	}

	if cfg.Job.Agent != "gpt-5.2-codex" {
		t.Fatalf("expected agent %q, got %q", "gpt-5.2-codex", cfg.Job.Agent)
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

func TestLoad_UsesGlobalWhenProjectMissing(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `
[workspace]
on-create = "global create"

[job]
agent = "global-agent"
test-commands = ["go test ./..."]
`

	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	repoDir := t.TempDir()
	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate != "global create" {
		t.Errorf("OnCreate = %q, expected %q", cfg.Workspace.OnCreate, "global create")
	}
	if cfg.Job.Agent != "global-agent" {
		t.Errorf("Agent = %q, expected %q", cfg.Job.Agent, "global-agent")
	}
	if len(cfg.Job.TestCommands) != 1 || cfg.Job.TestCommands[0] != "go test ./..." {
		t.Fatalf("expected global test commands to load")
	}
}

func TestLoad_ProjectOverridesGlobal(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	globalContent := `
[workspace]
on-create = "global create"

[job]
agent = "global-agent"
test-commands = ["global command"]
`
	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	projectContent := `
[workspace]
on-acquire = "project acquire"

[job]
agent = "project-agent"
test-commands = ["project command"]
`

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "incrementum.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate != "global create" {
		t.Errorf("OnCreate = %q, expected %q", cfg.Workspace.OnCreate, "global create")
	}
	if cfg.Workspace.OnAcquire != "project acquire" {
		t.Errorf("OnAcquire = %q, expected %q", cfg.Workspace.OnAcquire, "project acquire")
	}
	if cfg.Job.Agent != "project-agent" {
		t.Errorf("Agent = %q, expected %q", cfg.Job.Agent, "project-agent")
	}
	if len(cfg.Job.TestCommands) != 1 || cfg.Job.TestCommands[0] != "project command" {
		t.Fatalf("expected project test commands to override global")
	}
}

func TestLoad_ProjectEmptyOverridesGlobal(t *testing.T) {
	homeDir := testsupport.SetupTestHome(t)
	configDir := filepath.Join(homeDir, ".config", "incrementum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	globalContent := `
[workspace]
on-create = "global create"
on-acquire = "global acquire"

[job]
agent = "global-agent"
test-commands = ["global command"]
`
	globalPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalContent), 0o644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	projectContent := `
[workspace]
on-create = ""
on-acquire = ""

[job]
agent = ""
test-commands = []
`

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "incrementum.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Workspace.OnCreate != "" {
		t.Errorf("OnCreate = %q, expected empty string", cfg.Workspace.OnCreate)
	}
	if cfg.Workspace.OnAcquire != "" {
		t.Errorf("OnAcquire = %q, expected empty string", cfg.Workspace.OnAcquire)
	}
	if cfg.Job.Agent != "" {
		t.Errorf("Agent = %q, expected empty string", cfg.Job.Agent)
	}
	if len(cfg.Job.TestCommands) != 0 {
		t.Fatalf("expected empty test commands, got %d", len(cfg.Job.TestCommands))
	}
}
