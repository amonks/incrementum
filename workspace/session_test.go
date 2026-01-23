package workspace

import (
	"errors"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestPool_CreateSessionAndFind(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	session, err := pool.CreateSession(repoPath, "abc12345", "ws-001", "Test topic", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if session.ID == "" {
		t.Fatal("expected session ID")
	}
	if len(session.ID) != 8 {
		t.Fatalf("expected 8-char session ID, got %q", session.ID)
	}
	if session.Status != SessionActive {
		t.Fatalf("expected status active, got %q", session.Status)
	}
	if session.Repo != statestore.SanitizeRepoName(repoPath) {
		t.Fatalf("expected repo %q, got %q", statestore.SanitizeRepoName(repoPath), session.Repo)
	}
	if session.TodoID != "abc12345" {
		t.Fatalf("expected todo ID abc12345, got %q", session.TodoID)
	}
	if session.WorkspaceName != "ws-001" {
		t.Fatalf("expected workspace ws-001, got %q", session.WorkspaceName)
	}
	if session.Topic != "Test topic" {
		t.Fatalf("expected topic "+"Test topic"+", got %q", session.Topic)
	}
	if session.ExitCode != nil {
		t.Fatalf("expected nil exit code, got %v", *session.ExitCode)
	}
	if session.StartedAt.IsZero() {
		t.Fatal("expected started_at to be set")
	}
	if session.UpdatedAt.IsZero() {
		t.Fatal("expected updated_at to be set")
	}

	byTodo, err := pool.FindActiveSessionByTodoID(repoPath, "abc12345")
	if err != nil {
		t.Fatalf("find by todo: %v", err)
	}
	if byTodo.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, byTodo.ID)
	}

	byWorkspace, err := pool.FindActiveSessionByWorkspace(repoPath, "ws-001")
	if err != nil {
		t.Fatalf("find by workspace: %v", err)
	}
	if byWorkspace.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, byWorkspace.ID)
	}

	list, err := pool.ListSessions(repoPath)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, list[0].ID)
	}
}

func TestPool_CreateSession_ActiveAlreadyExists(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	_, err = pool.CreateSession(repoPath, "abc12345", "ws-001", "Test topic", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = pool.CreateSession(repoPath, "abc12345", "ws-002", "Other topic", start.Add(time.Minute))
	if !errors.Is(err, ErrSessionAlreadyActive) {
		t.Fatalf("expected ErrSessionAlreadyActive, got %v", err)
	}
}

func TestPool_CompleteSession(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	session, err := pool.CreateSession(repoPath, "abc12345", "ws-001", "Test topic", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	completedAt := start.Add(2 * time.Minute)
	exitCode := 0
	duration := int(completedAt.Sub(start).Seconds())

	completed, err := pool.CompleteSession(repoPath, session.ID, SessionCompleted, completedAt, &exitCode, duration)
	if err != nil {
		t.Fatalf("complete session: %v", err)
	}

	if completed.Status != SessionCompleted {
		t.Fatalf("expected status completed, got %q", completed.Status)
	}
	if completed.CompletedAt.IsZero() {
		t.Fatal("expected completed_at to be set")
	}
	if !completed.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completed_at %v, got %v", completedAt, completed.CompletedAt)
	}
	if completed.DurationSeconds != duration {
		t.Fatalf("expected duration %d, got %d", duration, completed.DurationSeconds)
	}
	if completed.ExitCode == nil || *completed.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %v", completed.ExitCode)
	}

	_, err = pool.FindActiveSessionByTodoID(repoPath, "abc12345")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}

	_, err = pool.CompleteSession(repoPath, session.ID, SessionCompleted, completedAt, &exitCode, duration)
	if !errors.Is(err, ErrSessionNotActive) {
		t.Fatalf("expected ErrSessionNotActive, got %v", err)
	}
}
