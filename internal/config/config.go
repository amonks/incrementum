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

// Load loads the incrementum.toml configuration from the given directory.
// Returns an empty config if the file doesn't exist.
func Load(repoPath string) (*Config, error) {
	configPath := filepath.Join(repoPath, "incrementum.toml")

	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
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
