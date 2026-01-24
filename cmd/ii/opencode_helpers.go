package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/amonks/incrementum/opencode"
	"github.com/amonks/incrementum/workspace"
)

func filterOpencodeSessionsForList(sessions []opencode.OpencodeSession, includeAll bool) []opencode.OpencodeSession {
	if includeAll {
		return sessions
	}

	filtered := make([]opencode.OpencodeSession, 0, len(sessions))
	for _, session := range sessions {
		if session.Status != opencode.OpencodeSessionActive {
			continue
		}
		filtered = append(filtered, session)
	}
	return filtered
}

func getOpencodeRepoPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
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
