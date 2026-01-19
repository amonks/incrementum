// Package main implements the incr CLI tool.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "incr",
	Short: "Incrementum - tools for incremental development",
}

// getRepoPath returns the jj repository root for the current directory.
func getRepoPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	root, err := workspace.RepoRootFromPath(cwd)
	if err != nil {
		if errors.Is(err, workspace.ErrWorkspaceRootNotFound) {
			return "", fmt.Errorf("not in a jj repository: %w", err)
		}
		if errors.Is(err, workspace.ErrRepoPathNotFound) {
			return "", fmt.Errorf("workspace repo mapping missing: %w", err)
		}
		return "", err
	}

	return root, nil
}

// resolvePath returns the workspace path from args or current directory.
func resolvePath(args []string) (string, error) {
	if len(args) > 0 {
		path := args[0]
		if !filepath.IsAbs(path) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("get working directory: %w", err)
			}
			path = filepath.Join(cwd, path)
		}
		return path, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return cwd, nil
}

// resolveWorkspaceName returns the workspace name from args or current directory.
func resolveWorkspaceName(args []string, pool *workspace.Pool) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	return pool.WorkspaceNameForPath(cwd)
}
