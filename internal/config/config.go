// Package config handles loading incrementum.toml configuration files.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the incrementum.toml configuration file.
type Config struct {
	Workspace Workspace `toml:"workspace"`
	Job       Job       `toml:"job"`
}

// Workspace contains workspace-related configuration.
type Workspace struct {
	// OnCreate is a script to run when a workspace is first created.
	// Can include a shebang line; defaults to bash if not specified.
	OnCreate string `toml:"on-create"`

	// OnAcquire is a script to run every time a workspace is acquired.
	// Can include a shebang line; defaults to bash if not specified.
	OnAcquire string `toml:"on-acquire"`
}

// Job contains job-related configuration.
type Job struct {
	// TestCommands defines commands to run during job testing.
	TestCommands []string `toml:"test-commands"`
	// Agent selects the default opencode agent for job runs.
	Agent string `toml:"agent"`
}

// Load loads configuration from the repo root and the global config file.
// Returns an empty config if no config files exist.
func Load(repoPath string) (*Config, error) {
	globalPath, err := globalConfigPath()
	if err != nil {
		return nil, err
	}

	globalCfg, globalMeta, err := loadConfigFile(globalPath)
	if err != nil {
		return nil, err
	}

	projectCfg, projectMeta, err := loadConfigFile(filepath.Join(repoPath, "incrementum.toml"))
	if err != nil {
		return nil, err
	}

	merged := mergeConfigs(globalCfg, projectCfg, globalMeta, projectMeta)
	return merged, nil
}

func globalConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "incrementum", "config.toml"), nil
}

func loadConfigFile(path string) (*Config, toml.MetaData, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, toml.MetaData{}, nil
	}
	if err != nil {
		return nil, toml.MetaData{}, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, toml.MetaData{}, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return &cfg, meta, nil
}

func mergeConfigs(globalCfg, projectCfg *Config, globalMeta, projectMeta toml.MetaData) *Config {
	if globalCfg == nil {
		globalCfg = &Config{}
	}
	if projectCfg == nil {
		projectCfg = &Config{}
	}

	merged := Config{}
	merged.Workspace.OnCreate = mergeString(projectMeta.IsDefined("workspace", "on-create"), projectCfg.Workspace.OnCreate, globalCfg.Workspace.OnCreate)
	merged.Workspace.OnAcquire = mergeString(projectMeta.IsDefined("workspace", "on-acquire"), projectCfg.Workspace.OnAcquire, globalCfg.Workspace.OnAcquire)
	merged.Job.Agent = mergeString(projectMeta.IsDefined("job", "agent"), projectCfg.Job.Agent, globalCfg.Job.Agent)
	if projectMeta.IsDefined("job", "test-commands") {
		merged.Job.TestCommands = append([]string(nil), projectCfg.Job.TestCommands...)
	} else if globalMeta.IsDefined("job", "test-commands") {
		merged.Job.TestCommands = append([]string(nil), globalCfg.Job.TestCommands...)
	}

	return &merged
}

func mergeString(projectDefined bool, projectValue, globalValue string) string {
	value := globalValue
	if projectDefined {
		value = projectValue
	}
	return strings.TrimSpace(value)
}

// RunScript executes a script in the given directory.
// If the script starts with a shebang (#!), that interpreter is used.
// Otherwise, the script is run with /bin/bash.
func RunScript(dir, script string) error {
	script = strings.TrimSpace(script)
	if script == "" {
		return nil
	}

	var interpreter string
	var scriptBody string

	if strings.HasPrefix(script, "#!") {
		// Extract shebang line
		lines := strings.SplitN(script, "\n", 2)
		interpreter = strings.TrimPrefix(lines[0], "#!")
		interpreter = strings.TrimSpace(interpreter)
		if len(lines) > 1 {
			scriptBody = lines[1]
		} else {
			scriptBody = ""
		}
	} else {
		interpreter = "/bin/bash"
		scriptBody = script
	}

	// Parse interpreter and args (e.g., "/usr/bin/env python3" or "/bin/bash -e")
	parts := strings.Fields(interpreter)
	if len(parts) == 0 {
		return fmt.Errorf("empty interpreter in shebang")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(scriptBody)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
