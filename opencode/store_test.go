package opencode

import (
	"errors"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestStore_CreateSessionAndList(t *testing.T) {
	store := openTestStore(t)

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	sessionID := "ses_test"
	session, err := store.CreateSession(repoPath, sessionID, start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if session.ID != sessionID {
		t.Fatalf("expected session ID %q, got %q", sessionID, session.ID)
	}
	if session.Status != OpencodeSessionActive {
		t.Fatalf("expected status active, got %q", session.Status)
	}
	if session.Repo != statestore.SanitizeRepoName(repoPath) {
		t.Fatalf("expected repo %q, got %q", statestore.SanitizeRepoName(repoPath), session.Repo)
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
	if !session.CompletedAt.IsZero() {
		t.Fatal("expected completed_at to be zero")
	}

	found, err := store.FindSession(repoPath, session.ID)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if found.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, found.ID)
	}

	list, err := store.ListSessions(repoPath)
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

func TestStore_FindSessionMatchesPrefix(t *testing.T) {
	store := openTestStore(t)

	repoPath := "/tmp/my-repo"
	startedAt := time.Now().UTC()

	session, err := store.CreateSession(repoPath, "alpha123", startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}

	_, err = store.CreateSession(repoPath, "beta456", startedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	prefix := session.ID[:3]
	found, err := store.FindSession(repoPath, prefix)
	if err != nil {
		t.Fatalf("find session by prefix: %v", err)
	}
	if found.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, found.ID)
	}
}

func TestStore_FindSessionRejectsAmbiguousPrefix(t *testing.T) {
	store := openTestStore(t)

	repoPath := "/tmp/my-repo"
	startedAt := time.Now().UTC()

	first, err := store.CreateSession(repoPath, "alpha123", startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}

	second, err := store.CreateSession(repoPath, "alpha456", startedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	commonPrefix := ""
	for i := 0; i < len(first.ID) && i < len(second.ID); i++ {
		if first.ID[i] == second.ID[i] {
			commonPrefix += string(first.ID[i])
		} else {
			break
		}
	}

	if commonPrefix != "" {
		_, err = store.FindSession(repoPath, commonPrefix)
		if !errors.Is(err, ErrAmbiguousOpencodeSessionIDPrefix) {
			t.Fatalf("expected ErrAmbiguousOpencodeSessionIDPrefix for common prefix %q, got %v", commonPrefix, err)
		}
	}
}

func TestStore_CompleteSession(t *testing.T) {
	store := openTestStore(t)

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	session, err := store.CreateSession(repoPath, "ses_complete", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	completedAt := start.Add(2 * time.Minute)
	exitCode := 1
	duration := int(completedAt.Sub(start).Seconds())

	completed, err := store.CompleteSession(repoPath, session.ID, OpencodeSessionFailed, completedAt, &exitCode, duration)
	if err != nil {
		t.Fatalf("complete session: %v", err)
	}

	if completed.Status != OpencodeSessionFailed {
		t.Fatalf("expected status failed, got %q", completed.Status)
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
	if completed.ExitCode == nil || *completed.ExitCode != exitCode {
		t.Fatalf("expected exit code %d, got %v", exitCode, completed.ExitCode)
	}

	_, err = store.CompleteSession(repoPath, session.ID, OpencodeSessionFailed, completedAt, &exitCode, duration)
	if !errors.Is(err, ErrOpencodeSessionNotActive) {
		t.Fatalf("expected ErrOpencodeSessionNotActive, got %v", err)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenWithOptions(Options{
		StateDir:    t.TempDir(),
		StorageRoot: t.TempDir(),
		EventsDir:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}
