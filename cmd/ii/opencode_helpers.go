package main

import "github.com/amonks/incrementum/workspace"

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
