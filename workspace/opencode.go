package workspace

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrOpencodeSessionNotFound indicates the requested session is missing.
var ErrOpencodeSessionNotFound = errors.New("opencode session not found")

// ErrOpencodeSessionNotActive indicates a session is not active.
var ErrOpencodeSessionNotActive = errors.New("opencode session is not active")

// CreateOpencodeSession creates a new active opencode session.
func (p *Pool) CreateOpencodeSession(repoPath, prompt, logPath string, startedAt time.Time) (OpencodeSession, error) {
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("get repo name: %w", err)
	}

	var created OpencodeSession

	err = p.stateStore.update(func(st *state) error {
		sessionID := generateOpencodeSessionID(prompt, startedAt)
		created = OpencodeSession{
			ID:        sessionID,
			Repo:      repoName,
			Status:    OpencodeSessionActive,
			Prompt:    prompt,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
			LogPath:   logPath,
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
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.load()
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("load state: %w", err)
	}

	key := repoName + "/" + sessionID
	session, ok := st.OpencodeSessions[key]
	if !ok {
		return OpencodeSession{}, ErrOpencodeSessionNotFound
	}

	return session, nil
}

// ListOpencodeSessions returns all sessions for a repo.
func (p *Pool) ListOpencodeSessions(repoPath string) ([]OpencodeSession, error) {
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.load()
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

	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("get repo name: %w", err)
	}

	var updated OpencodeSession

	err = p.stateStore.update(func(st *state) error {
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

// GenerateOpencodeSessionID returns the deterministic session id for a prompt/time.
func GenerateOpencodeSessionID(prompt string, startedAt time.Time) string {
	return generateOpencodeSessionID(prompt, startedAt)
}

func generateOpencodeSessionID(prompt string, startedAt time.Time) string {
	input := prompt + startedAt.Format(time.RFC3339Nano)
	hash := sha256.Sum256([]byte(input))
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	return strings.ToLower(encoded[:10])
}
