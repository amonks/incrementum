package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the .incr.toml configuration file.
type Config struct {
	Workspace ConfigWorkspace `toml:"workspace"`
}

// ConfigWorkspace contains workspace-related configuration.
type ConfigWorkspace struct {
	// OnCreate specifies commands to run when a workspace is first created.
	OnCreate []string `toml:"on-create"`

	// OnAcquire specifies commands to run every time a workspace is acquired.
	OnAcquire []string `toml:"on-acquire"`
}

// LoadConfig loads the .incr.toml configuration from the given directory.
// Returns an empty config if the file doesn't exist.
func LoadConfig(repoPath string) (*Config, error) {
	configPath := filepath.Join(repoPath, ".incr.toml")

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
