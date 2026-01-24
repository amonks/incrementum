package main

import (
	"fmt"
	"os"

	internalopencode "github.com/amonks/incrementum/internal/opencode"
	"github.com/amonks/incrementum/workspace"
)

func filterOpencodeSessionsForList(sessions []workspace.OpencodeSession, includeAll bool) []workspace.OpencodeSession {
	if includeAll {
		return sessions
	}

	filtered := make([]workspace.OpencodeSession, 0, len(sessions))
	for _, session := range sessions {
		if session.Status != workspace.OpencodeSessionActive {
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
	return cwd, nil
}

func opencodeStorage() (internalopencode.Storage, error) {
	root, err := internalopencode.DefaultRoot()
	if err != nil {
		return internalopencode.Storage{}, err
	}
	return internalopencode.Storage{Root: root}, nil
}
