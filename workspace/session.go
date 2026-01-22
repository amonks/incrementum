package workspace

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

// SessionStatus represents the state of a session.
type SessionStatus = statestore.SessionStatus

const (
	// SessionActive indicates the session is still active.
	SessionActive SessionStatus = statestore.SessionActive
	// SessionCompleted indicates the session completed successfully.
	SessionCompleted SessionStatus = statestore.SessionCompleted
	// SessionFailed indicates the session failed.
	SessionFailed SessionStatus = statestore.SessionFailed
)

// ValidSessionStatuses returns all valid session status values.
func ValidSessionStatuses() []SessionStatus {
	return statestore.ValidSessionStatuses()
}

// Session represents an active or completed session.
type Session = statestore.Session

// ErrSessionAlreadyActive indicates a todo already has an active session.
var ErrSessionAlreadyActive = errors.New("session already active")

// ErrSessionNotFound indicates the requested session is missing.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionNotActive indicates a session is not active.
var ErrSessionNotActive = errors.New("session is not active")

// CreateSession creates a new active session for the given todo.
func (p *Pool) CreateSession(repoPath, todoID, workspaceName, topic string, startedAt time.Time) (Session, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return Session{}, fmt.Errorf("get repo name: %w", err)
	}

	var created Session

	err = p.stateStore.Update(func(st *statestore.State) error {
		for _, session := range st.Sessions {
			if session.Repo == repoName && session.TodoID == todoID && session.Status == SessionActive {
				return ErrSessionAlreadyActive
			}
		}

		sessionID := generateSessionID(todoID, startedAt)
		created = Session{
			ID:            sessionID,
			Repo:          repoName,
			TodoID:        todoID,
			WorkspaceName: workspaceName,
			Status:        SessionActive,
			Topic:         topic,
			StartedAt:     startedAt,
			UpdatedAt:     startedAt,
		}

		st.Sessions[repoName+"/"+sessionID] = created
		return nil
	})

	if err != nil {
		return Session{}, err
	}

	return created, nil
}

// FindActiveSessionByTodoID returns the active session for a todo.
func (p *Pool) FindActiveSessionByTodoID(repoPath, todoID string) (Session, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return Session{}, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
	if err != nil {
		return Session{}, fmt.Errorf("load state: %w", err)
	}

	for _, session := range st.Sessions {
		if session.Repo == repoName && session.TodoID == todoID && session.Status == SessionActive {
			return session, nil
		}
	}

	return Session{}, ErrSessionNotFound
}

// FindActiveSessionByWorkspace returns the active session for a workspace.
func (p *Pool) FindActiveSessionByWorkspace(repoPath, workspaceName string) (Session, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return Session{}, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
	if err != nil {
		return Session{}, fmt.Errorf("load state: %w", err)
	}

	for _, session := range st.Sessions {
		if session.Repo == repoName && session.WorkspaceName == workspaceName && session.Status == SessionActive {
			return session, nil
		}
	}

	return Session{}, ErrSessionNotFound
}

// CompleteSession updates a session to completed or failed.
func (p *Pool) CompleteSession(repoPath, sessionID string, status SessionStatus, completedAt time.Time, exitCode *int, durationSeconds int) (Session, error) {
	if status != SessionCompleted && status != SessionFailed {
		return Session{}, fmt.Errorf("invalid session status: %s", status)
	}

	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return Session{}, fmt.Errorf("get repo name: %w", err)
	}

	var updated Session

	err = p.stateStore.Update(func(st *statestore.State) error {
		key := repoName + "/" + sessionID
		session, ok := st.Sessions[key]
		if !ok {
			return ErrSessionNotFound
		}
		if session.Status != SessionActive {
			return ErrSessionNotActive
		}

		session.Status = status
		session.CompletedAt = completedAt
		session.UpdatedAt = completedAt
		session.ExitCode = exitCode
		session.DurationSeconds = durationSeconds
		st.Sessions[key] = session
		updated = session
		return nil
	})

	if err != nil {
		return Session{}, err
	}

	return updated, nil
}

// ListSessions returns all sessions for a repo.
func (p *Pool) ListSessions(repoPath string) ([]Session, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	items := make([]Session, 0)
	for _, session := range st.Sessions {
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

func generateSessionID(todoID string, startedAt time.Time) string {
	input := todoID + startedAt.Format(time.RFC3339Nano)
	hash := sha256.Sum256([]byte(input))
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	return strings.ToLower(encoded[:10])
}
