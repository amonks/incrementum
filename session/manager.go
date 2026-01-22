package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/incrementum/todo"
	"github.com/amonks/incrementum/workspace"
)

// OpenOptions configures the session manager.
type OpenOptions struct {
	Todo      todo.OpenOptions
	Workspace workspace.Options
	// AllowMissingTodo permits opening without a todo store.
	AllowMissingTodo bool
}

// StartOptions configures a session start.
type StartOptions struct {
	Topic string
	Rev   string
}

// FinalizeOptions configures done/fail.
type FinalizeOptions struct {
	WorkspacePath string
}

// RunOptions configures a session run.
type RunOptions struct {
	Command []string
	Rev     string
}

// StartResult captures the output of starting a session.
type StartResult struct {
	Session       Session
	WorkspacePath string
	RepoPath      string
}

// RunResult captures output of a run session.
type RunResult struct {
	Session       Session
	WorkspacePath string
	RepoPath      string
	ExitCode      int
}

// Manager coordinates todo/workspace interactions for sessions.
type Manager struct {
	repoPath      string
	store         *todo.Store
	pool          *workspace.Pool
	createSession func(repoPath, todoID, workspaceName, topic string, startedAt time.Time) (workspace.Session, error)
}

// Open creates a new session manager for a repo.
func Open(repoPath string, opts OpenOptions) (*Manager, error) {
	store, err := todo.Open(repoPath, opts.Todo)
	if err != nil {
		if !opts.AllowMissingTodo || !errors.Is(err, todo.ErrNoTodoStore) {
			return nil, err
		}
		store = nil
	}

	pool, err := workspace.OpenWithOptions(opts.Workspace)
	if err != nil {
		if store != nil {
			store.Release()
		}
		return nil, err
	}

	return &Manager{
		repoPath:      repoPath,
		store:         store,
		pool:          pool,
		createSession: pool.CreateSession,
	}, nil
}

// Close releases resources held by the manager.
func (m *Manager) Close() error {
	if m.store == nil {
		return nil
	}
	return m.store.Release()
}

// Start starts a session for a todo.
func (m *Manager) Start(todoID string, opts StartOptions) (*StartResult, error) {
	if err := m.requireStore(); err != nil {
		return nil, err
	}
	item, err := m.resolveTodo(todoID)
	if err != nil {
		return nil, err
	}
	if err := validateTodoForSessionStart(item); err != nil {
		return nil, err
	}

	if _, err := m.pool.FindActiveSessionByTodoID(m.repoPath, item.ID); err == nil {
		return nil, ErrSessionAlreadyActive
	} else if !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}

	purpose := strings.TrimSpace(opts.Topic)
	if purpose == "" {
		purpose = item.Title
	}

	wsPath, err := m.pool.Acquire(m.repoPath, workspace.AcquireOptions{Rev: opts.Rev, Purpose: purpose})
	if err != nil {
		return nil, fmt.Errorf("acquire workspace: %w", err)
	}

	wsName := filepath.Base(wsPath)

	status := todo.StatusInProgress
	if _, err := m.store.Update([]string{item.ID}, todo.UpdateOptions{Status: &status}); err != nil {
		_ = m.pool.Release(wsPath)
		return nil, err
	}

	topic := purpose

	startedAt := time.Now()
	created, err := m.createSession(m.repoPath, item.ID, wsName, topic, startedAt)
	if err != nil {
		reset := todo.StatusOpen
		_, _ = m.store.Update([]string{item.ID}, todo.UpdateOptions{Status: &reset})
		_ = m.pool.Release(wsPath)
		return nil, err
	}

	return &StartResult{
		Session:       fromWorkspaceSession(created),
		WorkspacePath: wsPath,
		RepoPath:      m.repoPath,
	}, nil
}

// Done marks a session completed.
func (m *Manager) Done(todoID string, opts FinalizeOptions) (*Session, error) {
	return m.finalize(todoID, opts, todo.StatusDone, workspace.SessionCompleted)
}

// Fail marks a session failed.
func (m *Manager) Fail(todoID string, opts FinalizeOptions) (*Session, error) {
	return m.finalize(todoID, opts, todo.StatusOpen, workspace.SessionFailed)
}

// Run executes a command in a session workspace.
func (m *Manager) Run(todoID string, opts RunOptions) (*RunResult, error) {
	if err := m.requireStore(); err != nil {
		return nil, err
	}
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	item, err := m.resolveTodo(todoID)
	if err != nil {
		return nil, err
	}
	if err := validateTodoForSessionStart(item); err != nil {
		return nil, err
	}

	if _, err := m.pool.FindActiveSessionByTodoID(m.repoPath, item.ID); err == nil {
		return nil, ErrSessionAlreadyActive
	} else if !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}

	purpose := strings.Join(opts.Command, " ")
	wsPath, err := m.pool.Acquire(m.repoPath, workspace.AcquireOptions{Rev: opts.Rev, Purpose: purpose})
	if err != nil {
		return nil, fmt.Errorf("acquire workspace: %w", err)
	}

	wsName := filepath.Base(wsPath)

	status := todo.StatusInProgress
	if _, err := m.store.Update([]string{item.ID}, todo.UpdateOptions{Status: &status}); err != nil {
		m.pool.Release(wsPath)
		return nil, err
	}

	released := false
	releaseWorkspace := func() error {
		if released {
			return nil
		}
		if err := m.pool.ReleaseByName(m.repoPath, wsName); err != nil {
			return err
		}
		released = true
		return nil
	}
	defer func() {
		if !released {
			_ = releaseWorkspace()
		}
	}()

	startedAt := time.Now()
	topic := purpose
	created, err := m.createSession(m.repoPath, item.ID, wsName, topic, startedAt)
	if err != nil {
		reset := todo.StatusOpen
		_, _ = m.store.Update([]string{item.ID}, todo.UpdateOptions{Status: &reset})
		return nil, err
	}

	execCmd := exec.Command(opts.Command[0], opts.Command[1:]...)
	execCmd.Dir = wsPath
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	runErr := execCmd.Run()
	completedAt := time.Now()
	duration := int(completedAt.Sub(startedAt).Seconds())

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	finalStatus := todo.StatusDone
	sessionStatus := workspace.SessionCompleted
	if runErr != nil {
		finalStatus = todo.StatusOpen
		sessionStatus = workspace.SessionFailed
	}

	if _, err := m.store.Update([]string{item.ID}, todo.UpdateOptions{Status: &finalStatus}); err != nil {
		return nil, err
	}

	completed, err := m.pool.CompleteSession(m.repoPath, created.ID, sessionStatus, completedAt, &exitCode, duration)
	if err != nil {
		return nil, err
	}

	if err := releaseWorkspace(); err != nil {
		return nil, err
	}

	if runErr != nil {
		return &RunResult{
			Session:       fromWorkspaceSession(completed),
			WorkspacePath: wsPath,
			RepoPath:      m.repoPath,
			ExitCode:      exitCode,
		}, runErr
	}

	return &RunResult{
		Session:       fromWorkspaceSession(completed),
		WorkspacePath: wsPath,
		RepoPath:      m.repoPath,
		ExitCode:      exitCode,
	}, nil
}

// ListFilter configures which sessions to return.
type ListFilter struct {
	// Status filters by exact status match.
	Status *Status
	// IncludeAll includes sessions regardless of status.
	IncludeAll bool
}

// List returns sessions for the repo.
func (m *Manager) List(filter ListFilter) ([]Session, error) {
	if filter.Status != nil && !filter.Status.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, *filter.Status)
	}

	items, err := m.pool.ListSessions(m.repoPath)
	if err != nil {
		return nil, err
	}

	results := make([]Session, 0, len(items))
	for _, item := range items {
		converted := fromWorkspaceSession(item)
		if filter.Status != nil {
			if converted.Status != *filter.Status {
				continue
			}
		} else if !filter.IncludeAll && converted.Status != StatusActive {
			continue
		}
		results = append(results, converted)
	}
	return results, nil
}

// TodoIDPrefixLengths returns prefix lengths for todo IDs in the store.
func (m *Manager) TodoIDPrefixLengths() (map[string]int, error) {
	if m.store == nil {
		return nil, nil
	}
	index, err := m.store.IDIndex()
	if err != nil {
		return nil, err
	}
	return index.PrefixLengths(), nil
}

// ResolveActiveSession finds an active session by todo ID or workspace path.
func (m *Manager) ResolveActiveSession(todoID, workspacePath string) (*Session, error) {
	if todoID != "" {
		return m.resolveActiveSessionByTodoPrefix(todoID)
	}

	if workspacePath == "" {
		return nil, fmt.Errorf("todo id required when not in a workspace")
	}

	wsName, err := m.pool.WorkspaceNameForPath(workspacePath)
	if err != nil {
		if errors.Is(err, workspace.ErrWorkspaceRootNotFound) || errors.Is(err, workspace.ErrRepoPathNotFound) {
			return nil, fmt.Errorf("todo id required when not in a workspace")
		}
		return nil, err
	}

	found, err := m.pool.FindActiveSessionByWorkspace(m.repoPath, wsName)
	if err != nil {
		return nil, err
	}
	converted := fromWorkspaceSession(found)
	return &converted, nil
}

func (m *Manager) resolveActiveSessionByTodoPrefix(todoID string) (*Session, error) {
	items, err := m.pool.ListSessions(m.repoPath)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(todoID)
	var matched workspace.Session
	found := false

	for _, item := range items {
		if item.Status != workspace.SessionActive {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(item.TodoID), needle) {
			continue
		}
		if found && !strings.EqualFold(matched.TodoID, item.TodoID) {
			return nil, fmt.Errorf("%w: %s", todo.ErrAmbiguousTodoIDPrefix, todoID)
		}
		matched = item
		found = true
	}

	if !found {
		return nil, ErrSessionNotFound
	}

	converted := fromWorkspaceSession(matched)
	return &converted, nil
}

// Age computes the display age for a session.
func Age(item Session, now time.Time) time.Duration {
	if item.Status == StatusActive {
		if item.StartedAt.IsZero() {
			return 0
		}
		return now.Sub(item.StartedAt)
	}

	if item.DurationSeconds > 0 {
		return time.Duration(item.DurationSeconds) * time.Second
	}

	if !item.CompletedAt.IsZero() && !item.StartedAt.IsZero() {
		return item.CompletedAt.Sub(item.StartedAt)
	}

	return 0
}

func (m *Manager) finalize(todoID string, opts FinalizeOptions, todoStatus todo.Status, sessionStatus workspace.SessionStatus) (*Session, error) {
	if err := m.requireStore(); err != nil {
		return nil, err
	}
	resolved, err := m.ResolveActiveSession(todoID, opts.WorkspacePath)
	if err != nil {
		return nil, err
	}

	if err := m.pool.ReleaseByName(m.repoPath, resolved.WorkspaceName); err != nil {
		return nil, err
	}

	if _, err := m.store.Update([]string{resolved.TodoID}, todo.UpdateOptions{Status: &todoStatus}); err != nil {
		return nil, err
	}

	completedAt := time.Now()
	duration := int(completedAt.Sub(resolved.StartedAt).Seconds())
	updated, err := m.pool.CompleteSession(m.repoPath, resolved.ID, sessionStatus, completedAt, nil, duration)
	if err != nil {
		return nil, err
	}

	converted := fromWorkspaceSession(updated)
	return &converted, nil
}

func (m *Manager) resolveTodo(id string) (todo.Todo, error) {
	if err := m.requireStore(); err != nil {
		return todo.Todo{}, err
	}
	item, err := m.store.Show([]string{id})
	if err != nil {
		return todo.Todo{}, err
	}
	if len(item) == 0 {
		return todo.Todo{}, fmt.Errorf("todo not found: %s", id)
	}
	return item[0], nil
}

func validateTodoForSessionStart(item todo.Todo) error {
	switch item.Status {
	case todo.StatusInProgress, todo.StatusDone, todo.StatusClosed, todo.StatusTombstone:
		return fmt.Errorf("todo %s is not available (status %s)", item.ID, item.Status)
	default:
		return nil
	}
}

func fromWorkspaceSession(item workspace.Session) Session {
	return Session{
		ID:              item.ID,
		Repo:            item.Repo,
		TodoID:          item.TodoID,
		WorkspaceName:   item.WorkspaceName,
		Status:          Status(item.Status),
		Topic:           item.Topic,
		StartedAt:       item.StartedAt,
		UpdatedAt:       item.UpdatedAt,
		CompletedAt:     item.CompletedAt,
		ExitCode:        item.ExitCode,
		DurationSeconds: item.DurationSeconds,
	}
}

func (m *Manager) requireStore() error {
	if m.store == nil {
		return todo.ErrNoTodoStore
	}
	return nil
}
