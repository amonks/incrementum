package opencode

// Logs returns a snapshot of opencode session logs.
func (s *Store) Logs(repoPath, sessionID string) (string, error) {
	resolvedID, err := s.resolveSessionID(repoPath, sessionID)
	if err != nil {
		return "", err
	}
	return s.LogSnapshot(resolvedID)
}

// LogSnapshot returns the session transcript as a single string.
func (s *Store) LogSnapshot(sessionID string) (string, error) {
	return s.events.LogSnapshot(sessionID)
}

// TranscriptLogs returns a snapshot of the stored session transcript.
func (s *Store) TranscriptLogs(repoPath, sessionID string) (string, error) {
	resolvedID, err := s.resolveSessionID(repoPath, sessionID)
	if err != nil {
		return "", err
	}
	return s.TranscriptSnapshot(resolvedID)
}

// TranscriptSnapshot returns the session transcript including tool output.
func (s *Store) TranscriptSnapshot(sessionID string) (string, error) {
	return s.storage.SessionLogText(sessionID)
}

// ProseLogs returns a snapshot of session logs without tool output.
func (s *Store) ProseLogs(repoPath, sessionID string) (string, error) {
	resolvedID, err := s.resolveSessionID(repoPath, sessionID)
	if err != nil {
		return "", err
	}
	return s.ProseLogSnapshot(resolvedID)
}

func (s *Store) resolveSessionID(repoPath, sessionID string) (string, error) {
	session, err := s.FindSession(repoPath, sessionID)
	if err != nil {
		return "", err
	}
	return session.ID, nil
}

// ProseLogSnapshot returns the session transcript without tool output.
func (s *Store) ProseLogSnapshot(sessionID string) (string, error) {
	return s.storage.SessionProseLogText(sessionID)
}
