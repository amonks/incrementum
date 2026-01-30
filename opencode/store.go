package opencode

import (
	"fmt"
	"sort"
	"time"

	"github.com/amonks/incrementum/internal/ids"
	internalopencode "github.com/amonks/incrementum/internal/opencode"
	"github.com/amonks/incrementum/internal/paths"
	statestore "github.com/amonks/incrementum/internal/state"
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// Store manages opencode session state.
type Store struct {
	stateStore *statestore.Store
	storage    internalopencode.Storage
	events     eventStorage
}

// Options configures an opencode store.
type Options struct {
	// StateDir is the directory where state is stored.
	// Defaults to ~/.local/state/incrementum if empty.
	StateDir string

	// StorageRoot is the opencode storage root.
	// Defaults to ~/.local/share/opencode if empty.
	StorageRoot string

	// EventsDir is the directory where opencode events are stored.
	// Defaults to ~/.local/share/incrementum/opencode/events if empty.
	EventsDir string
}

// Open creates a store with default options.
func Open() (*Store, error) {
	return OpenWithOptions(Options{})
}

// OpenWithOptions creates a store with custom options.
func OpenWithOptions(opts Options) (*Store, error) {
	stateDir := opts.StateDir
	if stateDir == "" {
		var err error
		stateDir, err = paths.DefaultStateDir()
		if err != nil {
			return nil, err
		}
	}

	storageRoot := opts.StorageRoot
	if storageRoot == "" {
		var err error
		storageRoot, err = internalopencode.DefaultRoot()
		if err != nil {
			return nil, err
		}
	}

	eventsDir := opts.EventsDir
	if eventsDir == "" {
		var err error
		eventsDir, err = paths.DefaultOpencodeEventsDir()
		if err != nil {
			return nil, err
		}
	}

	return &Store{
		stateStore: statestore.NewStore(stateDir),
		storage:    internalopencode.Storage{Root: storageRoot},
		events:     eventStorage{Root: eventsDir},
	}, nil
}

func (s *Store) repoName(repoPath string) (string, error) {
	repoName, err := s.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return "", fmt.Errorf("get repo name: %w", err)
	}
	return repoName, nil
}

// CreateSession creates a new active opencode session.
func (s *Store) CreateSession(repoPath, sessionID string, startedAt time.Time) (OpencodeSession, error) {
	repoName, err := s.repoName(repoPath)
	if err != nil {
		return OpencodeSession{}, err
	}
	if internalstrings.IsBlank(sessionID) {
		return OpencodeSession{}, fmt.Errorf("session id is required")
	}

	var created OpencodeSession
	err = s.stateStore.Update(func(st *statestore.State) error {
		created = OpencodeSession{
			ID:        sessionID,
			Repo:      repoName,
			Status:    OpencodeSessionActive,
			CreatedAt: startedAt,
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

// FindSession returns the session with the given id in the repo.
func (s *Store) FindSession(repoPath, sessionID string) (OpencodeSession, error) {
	repoName, err := s.repoName(repoPath)
	if err != nil {
		return OpencodeSession{}, err
	}

	st, err := s.stateStore.Load()
	if err != nil {
		return OpencodeSession{}, fmt.Errorf("load state: %w", err)
	}

	if sessionID == "" {
		return OpencodeSession{}, ErrOpencodeSessionNotFound
	}

	sessionIDs := make([]string, 0, len(st.OpencodeSessions))
	sessionsByID := make(map[string]OpencodeSession, len(st.OpencodeSessions))
	for _, session := range st.OpencodeSessions {
		if session.Repo != repoName {
			continue
		}
		sessionIDs = append(sessionIDs, session.ID)
		sessionsByID[session.ID] = session
	}

	matchID, found, ambiguous := ids.MatchPrefix(sessionIDs, sessionID)
	if ambiguous {
		return OpencodeSession{}, fmt.Errorf("%w: %s", ErrAmbiguousOpencodeSessionIDPrefix, sessionID)
	}
	if !found {
		return OpencodeSession{}, ErrOpencodeSessionNotFound
	}

	return sessionsByID[matchID], nil
}

// ListSessions returns all sessions for a repo.
func (s *Store) ListSessions(repoPath string) ([]OpencodeSession, error) {
	repoName, err := s.repoName(repoPath)
	if err != nil {
		return nil, err
	}

	st, err := s.stateStore.Load()
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

// CompleteSession updates a session to completed, failed, or killed.
func (s *Store) CompleteSession(repoPath, sessionID string, status OpencodeSessionStatus, completedAt time.Time, exitCode *int, durationSeconds int) (OpencodeSession, error) {
	if status != OpencodeSessionCompleted && status != OpencodeSessionFailed && status != OpencodeSessionKilled {
		return OpencodeSession{}, fmt.Errorf("invalid opencode session status: %s", status)
	}

	repoName, err := s.repoName(repoPath)
	if err != nil {
		return OpencodeSession{}, err
	}

	var updated OpencodeSession
	err = s.stateStore.Update(func(st *statestore.State) error {
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
