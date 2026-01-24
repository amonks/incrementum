package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// WorkingDir returns the current working directory.
func WorkingDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return cwd, nil
}

// DefaultStateDir returns the default incrementum state directory.
func DefaultStateDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "state", "incrementum"), nil
}

// DefaultWorkspacesDir returns the default incrementum workspaces directory.
func DefaultWorkspacesDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "incrementum", "workspaces"), nil
}

// DefaultOpencodeEventsDir returns the default directory for opencode events.
func DefaultOpencodeEventsDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "incrementum", "opencode", "events"), nil
}

// DefaultJobEventsDir returns the default directory for job events.
func DefaultJobEventsDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "incrementum", "jobs", "events"), nil
}

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return home, nil
}
