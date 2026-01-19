// Package main implements the incr CLI tool.
package main

import (
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

	root, err := workspace.RepoRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("not in a jj repository: %w", err)
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
