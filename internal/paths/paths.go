package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorkingDir returns the current working directory.
func WorkingDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return normalizeWorkingDir(cwd), nil
}

func normalizeWorkingDir(cwd string) string {
	trimmed := strings.TrimPrefix(cwd, "/private")
	if trimmed == cwd {
		return cwd
	}
	originalInfo, err := os.Stat(cwd)
	if err != nil {
		return cwd
	}
	trimmedInfo, err := os.Stat(trimmed)
	if err != nil {
		return cwd
	}
	if os.SameFile(originalInfo, trimmedInfo) {
		return trimmed
	}
	return cwd
}

// DefaultStateDir returns the default incrementum state directory.
func DefaultStateDir() (string, error) {
	return defaultHomeDirPath(".local", "state", "incrementum")
}

// DefaultWorkspacesDir returns the default incrementum workspaces directory.
func DefaultWorkspacesDir() (string, error) {
	return defaultHomeDirPath(".local", "share", "incrementum", "workspaces")
}

// DefaultOpencodeEventsDir returns the default directory for opencode events.
func DefaultOpencodeEventsDir() (string, error) {
	return defaultHomeDirPath(".local", "share", "incrementum", "opencode", "events")
}

// DefaultJobEventsDir returns the default directory for job events.
func DefaultJobEventsDir() (string, error) {
	return defaultHomeDirPath(".local", "share", "incrementum", "jobs", "events")
}

// HomeDir returns the current user's home directory.
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return home, nil
}

func defaultHomeDirPath(parts ...string) (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(append([]string{home}, parts...)...), nil
}

// ResolveWithDefault returns the override value if non-empty, otherwise calls
// the default function to get a fallback value.
func ResolveWithDefault(override string, defaultFn func() (string, error)) (string, error) {
	if override != "" {
		return override, nil
	}
	return defaultFn()
}
