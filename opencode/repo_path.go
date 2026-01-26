package opencode

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/amonks/incrementum/workspace"
)

// RepoPathForWorkingDir resolves the repo path for the current directory.
//
// If the working directory is a workspace root, this resolves to the source repo.
// If no repo is found, it falls back to the working directory.
func RepoPathForWorkingDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cwd = filepath.Clean(cwd)

	repoPath, err := workspace.RepoRootFromPath(cwd)
	if err == nil {
		return repoPath, nil
	}
	if errors.Is(err, workspace.ErrWorkspaceRootNotFound) || errors.Is(err, workspace.ErrRepoPathNotFound) {
		return cwd, nil
	}
	return "", err
}
