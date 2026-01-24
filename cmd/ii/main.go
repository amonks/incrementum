// Package main implements the ii CLI tool.
package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/amonks/incrementum/internal/paths"
	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ii",
	Short: "Incrementum - tools for incremental development",
}

// getRepoPath returns the jj repository root for the current directory.
func getRepoPath() (string, error) {
	cwd, err := paths.WorkingDir()
	if err != nil {
		return "", err
	}

	return resolveRepoRoot(cwd)
}

// resolvePath returns the workspace path from args or current directory.
func resolvePath(args []string) (string, error) {
	if len(args) > 0 {
		path := args[0]
		if !filepath.IsAbs(path) {
			cwd, err := paths.WorkingDir()
			if err != nil {
				return "", err
			}
			path = filepath.Join(cwd, path)
		}
		return path, nil
	}

	cwd, err := paths.WorkingDir()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

// resolveWorkspaceName returns the workspace name from args or current directory.
func resolveWorkspaceName(args []string, pool *workspace.Pool) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	cwd, err := paths.WorkingDir()
	if err != nil {
		return "", err
	}

	return pool.WorkspaceNameForPath(cwd)
}
