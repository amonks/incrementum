package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultStateDir returns the default incrementum state directory.
func DefaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".local", "state", "incrementum"), nil
}

// DefaultWorkspacesDir returns the default incrementum workspaces directory.
func DefaultWorkspacesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".local", "share", "incrementum", "workspaces"), nil
}
