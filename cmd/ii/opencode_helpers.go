package main

import (
	"github.com/amonks/incrementum/opencode"
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

func drainOpencodeEvents(events <-chan opencode.Event) <-chan struct{} {
	done := make(chan struct{})
	if events == nil {
		close(done)
		return done
	}
	go func() {
		for range events {
		}
		close(done)
	}()
	return done
}
