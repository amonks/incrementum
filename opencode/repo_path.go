package opencode

import (
	"errors"

	"github.com/amonks/incrementum/internal/paths"
	"github.com/amonks/incrementum/workspace"
)

// RepoPathForWorkingDir resolves the repo path for the current directory.
//
// If the working directory is a workspace root, this resolves to the source repo.
// If no repo is found, it falls back to the working directory.
func RepoPathForWorkingDir() (string, error) {
	cwd, err := paths.WorkingDir()
	if err != nil {
		return "", err
	}

	repoPath, err := workspace.RepoRootFromPath(cwd)
	if err == nil {
		return repoPath, nil
	}
	if errors.Is(err, workspace.ErrWorkspaceRootNotFound) || errors.Is(err, workspace.ErrRepoPathNotFound) {
		return cwd, nil
	}
	return "", err
}
