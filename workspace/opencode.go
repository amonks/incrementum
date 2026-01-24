package workspace

import (
	"fmt"
	"sort"
	"strings"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

// CreateOpencodeSession creates a new active opencode session.
func (p *Pool) CreateOpencodeSession(repoPath, sessionID, prompt string, startedAt time.Time) (OpencodeSession, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("get repo name: %w", err)
	}
	if strings.TrimSpace(sessionID) == "" {
		return OpencodeSession{}, fmt.Errorf("session id is required")
	}

	var created OpencodeSession

	err = p.stateStore.Update(func(st *statestore.State) error {
		created = OpencodeSession{
			ID:        sessionID,
			Repo:      repoName,
			Status:    OpencodeSessionActive,
			Prompt:    prompt,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		}

		st.OpencodeSessions[repoName+"/"+sessionID] = created
		return nil
	})

	if err != nil {
		return OpencodeSession{}, err
	}

	return created, nil
}

// FindOpencodeSession returns the session with the given id in the repo.
func (p *Pool) FindOpencodeSession(repoPath, sessionID string) (OpencodeSession, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("load state: %w", err)
	}

	if sessionID == "" {
		return OpencodeSession{}, ErrOpencodeSessionNotFound
	}

	needle := strings.ToLower(sessionID)
	var match *OpencodeSession
	for _, session := range st.OpencodeSessions {
		if session.Repo != repoName {
			continue
		}
		idLower := strings.ToLower(session.ID)
		if idLower != needle && !strings.HasPrefix(idLower, needle) {
			continue
		}
		if match != nil && !strings.EqualFold(match.ID, session.ID) {
			return OpencodeSession{}, fmt.Errorf("%w: %s", ErrAmbiguousOpencodeSessionIDPrefix, sessionID)
		}
		matched := session
		match = &matched
	}

	if match == nil {
		return OpencodeSession{}, ErrOpencodeSessionNotFound
	}

	return *match, nil
}

// ListOpencodeSessions returns all sessions for a repo.
func (p *Pool) ListOpencodeSessions(repoPath string) ([]OpencodeSession, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	items := make([]OpencodeSession, 0)
	for _, session := range st.OpencodeSessions {
		if session.Repo == repoName {
			items = append(items, session)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})

	return items, nil
}

// CompleteOpencodeSession updates a session to completed, failed, or killed.
func (p *Pool) CompleteOpencodeSession(repoPath, sessionID string, status OpencodeSessionStatus, completedAt time.Time, exitCode *int, durationSeconds int) (OpencodeSession, error) {
	if status != OpencodeSessionCompleted && status != OpencodeSessionFailed && status != OpencodeSessionKilled {
		return OpencodeSession{}, fmt.Errorf("invalid opencode session status: %s", status)
	}

	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("get repo name: %w", err)
	}

	var updated OpencodeSession

	err = p.stateStore.Update(func(st *statestore.State) error {
		key := repoName + "/" + sessionID
		session, ok := st.OpencodeSessions[key]
		if !ok {
			return ErrOpencodeSessionNotFound
		}
		if session.Status != OpencodeSessionActive {
			return ErrOpencodeSessionNotActive
		}

		session.Status = status
		session.CompletedAt = completedAt
		session.UpdatedAt = completedAt
		session.ExitCode = exitCode
		session.DurationSeconds = durationSeconds
		st.OpencodeSessions[key] = session
		updated = session
		return nil
	})

	if err != nil {
		return OpencodeSession{}, err
	}

	return updated, nil
}
