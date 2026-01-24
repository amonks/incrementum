package opencode

// FilterSessionsForList returns the sessions to show in a list command.
func FilterSessionsForList(sessions []OpencodeSession, includeAll bool) []OpencodeSession {
	if includeAll {
		return sessions
	}

	filtered := make([]OpencodeSession, 0, len(sessions))
	for _, session := range sessions {
		if session.Status != OpencodeSessionActive {
			continue
		}
		filtered = append(filtered, session)
	}
	return filtered
}
