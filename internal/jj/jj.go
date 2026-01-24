// Package jj provides a wrapper around the jj CLI tool.
package jj

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps the jj CLI.
type Client struct{}

// ErrFileNotFound indicates a file or path is missing at a revision.
var ErrFileNotFound = errors.New("jj file not found")

// New creates a new jj client.
func New() *Client {
	return &Client{}
}

// Init initializes a new jj repository at the given path.
func (c *Client) Init(path string) error {
	cmd := exec.Command("jj", "git", "init")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj git init: %w: %s", err, output)
	}
	return nil
}

// WorkspaceRoot returns the root directory of the workspace containing the given path.
func (c *Client) WorkspaceRoot(path string) (string, error) {
	cmd := exec.Command("jj", "workspace", "root")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("jj workspace root: %w: %s", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("jj workspace root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// WorkspaceAdd adds a new workspace to the repository.
func (c *Client) WorkspaceAdd(repoPath, name, workspacePath string) error {
	cmd := exec.Command("jj", "workspace", "add", "--name", name, workspacePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj workspace add: %w: %s", err, output)
	}
	return nil
}

// WorkspaceList returns the list of workspace names in the repository.
func (c *Client) WorkspaceList(repoPath string) ([]string, error) {
	cmd := exec.Command("jj", "workspace", "list")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("jj workspace list: %w: %s", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("jj workspace list: %w", err)
	}

	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	workspaces := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		// Output format is "name: <change_id>" - extract just the name
		parts := bytes.SplitN(line, []byte(":"), 2)
		workspaces = append(workspaces, string(bytes.TrimSpace(parts[0])))
	}
	return workspaces, nil
}

// Edit checks out the specified revision in the workspace.
func (c *Client) Edit(workspacePath, rev string) error {
	cmd := exec.Command("jj", "edit", rev)
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj edit: %w: %s", err, output)
	}
	return nil
}

// CurrentChangeID returns the change ID of the current working copy commit.
func (c *Client) CurrentChangeID(workspacePath string) (string, error) {
	cmd := exec.Command("jj", "log", "-r", "@", "-T", "change_id", "--no-graph")
	cmd.Dir = workspacePath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("jj log: %w: %s", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("jj log: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// BookmarkList returns all bookmark names in the repository.
func (c *Client) BookmarkList(workspacePath string) ([]string, error) {
	cmd := exec.Command("jj", "bookmark", "list", "-T", "name ++ \"\\n\"")
	cmd.Dir = workspacePath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("jj bookmark list: %w: %s", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("jj bookmark list: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var bookmarks []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			bookmarks = append(bookmarks, line)
		}
	}
	return bookmarks, nil
}

// BookmarkCreate creates a bookmark at the specified revision.
func (c *Client) BookmarkCreate(workspacePath, name, rev string) error {
	cmd := exec.Command("jj", "bookmark", "create", name, "-r", rev)
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj bookmark create: %w: %s", err, output)
	}
	return nil
}

// NewChange creates a new change with the given parent revision.
// Returns the change ID of the newly created change.
// Note: This moves the working copy to the new change.
func (c *Client) NewChange(workspacePath, parentRev string) (string, error) {
	cmd := exec.Command("jj", "new", parentRev)
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("jj new: %w: %s", err, output)
	}

	// Get the change ID of the newly created change (now at @)
	return c.CurrentChangeID(workspacePath)
}

// ChangeIDAt returns the change ID at the given revision.
func (c *Client) ChangeIDAt(workspacePath, rev string) (string, error) {
	cmd := exec.Command("jj", "log", "-r", rev, "-T", "change_id", "--no-graph")
	cmd.Dir = workspacePath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("jj log: %w: %s", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("jj log: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Snapshot runs jj debug snapshot to record working copy changes to the current change.
func (c *Client) Snapshot(workspacePath string) error {
	cmd := exec.Command("jj", "debug", "snapshot")
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj debug snapshot: %w: %s", err, output)
	}
	return nil
}

// Describe sets the description for the current change.
func (c *Client) Describe(workspacePath, message string) error {
	cmd := exec.Command("jj", "describe", "-m", message)
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj describe: %w: %s", err, output)
	}
	return nil
}

// Commit commits the current change and leaves a new empty change.
func (c *Client) Commit(workspacePath, message string) error {
	cmd := exec.Command("jj", "commit", "-m", message)
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj commit: %w: %s", err, output)
	}
	return nil
}

// WorkspaceUpdateStale updates a stale working copy.
func (c *Client) WorkspaceUpdateStale(workspacePath string) error {
	cmd := exec.Command("jj", "workspace", "update-stale")
	cmd.Dir = workspacePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj workspace update-stale: %w: %s", err, output)
	}
	return nil
}

// WorkspaceForget removes a workspace from the repository without deleting it from disk.
func (c *Client) WorkspaceForget(repoPath, workspaceName string) error {
	cmd := exec.Command("jj", "workspace", "forget", workspaceName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jj workspace forget: %w: %s", err, output)
	}
	return nil
}

// FileShow returns the contents of a file at the given revision.
func (c *Client) FileShow(repoPath, rev, path string) ([]byte, error) {
	cmd := exec.Command("jj", "file", "show", "-r", rev, path)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isFileNotFoundOutput(output) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("jj file show: %w: %s", err, output)
	}
	return output, nil
}

func isFileNotFoundOutput(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "no such file") ||
		strings.Contains(message, "path does not exist") ||
		strings.Contains(message, "path doesn't exist")
}
