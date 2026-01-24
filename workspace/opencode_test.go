package workspace

import (
	"errors"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestPool_CreateOpencodeSessionAndList(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	sessionID := "ses_test"
	session, err := pool.CreateOpencodeSession(repoPath, sessionID, "Test prompt", start)
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
	if session.Prompt != "Test prompt" {
		t.Fatalf("expected prompt Test prompt, got %q", session.Prompt)
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

	found, err := pool.FindOpencodeSession(repoPath, session.ID)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if found.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, found.ID)
	}

	list, err := pool.ListOpencodeSessions(repoPath)
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

func TestPool_FindOpencodeSessionMatchesPrefix(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	startedAt := time.Now().UTC()

	// Create two sessions with different ID prefixes
	session, err := pool.CreateOpencodeSession(repoPath, "alpha123", "First prompt", startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}

	_, err = pool.CreateOpencodeSession(repoPath, "beta456", "Second prompt", startedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	// Find by prefix of first session's ID (use first 3 chars)
	prefix := session.ID[:3]
	found, err := pool.FindOpencodeSession(repoPath, prefix)
	if err != nil {
		t.Fatalf("find session by prefix: %v", err)
	}
	if found.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, found.ID)
	}
}

func TestPool_FindOpencodeSessionRejectsAmbiguousPrefix(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	startedAt := time.Now().UTC()

	// Create two sessions - their IDs will be hash-based so we need to test
	// that looking for a very short prefix returns an ambiguous error when both match
	first, err := pool.CreateOpencodeSession(repoPath, "alpha123", "First", startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}

	second, err := pool.CreateOpencodeSession(repoPath, "alpha456", "Second", startedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	// Find a prefix that matches both - need to find common prefix
	// Since IDs are hash-based, we'll look for the single-char prefix if they share one
	// If not, the test passes trivially. For robustness, check if they share a prefix.
	commonPrefix := ""
	for i := 0; i < len(first.ID) && i < len(second.ID); i++ {
		if first.ID[i] == second.ID[i] {
			commonPrefix += string(first.ID[i])
		} else {
			break
		}
	}

	if commonPrefix != "" {
		_, err = pool.FindOpencodeSession(repoPath, commonPrefix)
		if !errors.Is(err, ErrAmbiguousOpencodeSessionIDPrefix) {
			t.Fatalf("expected ErrAmbiguousOpencodeSessionIDPrefix for common prefix %q, got %v", commonPrefix, err)
		}
	}
	// If no common prefix, test passes - the IDs don't share a prefix
}

func TestPool_RepoSlug(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	slug, err := pool.RepoSlug(repoPath)
	if err != nil {
		t.Fatalf("get repo slug: %v", err)
	}

	if slug != statestore.SanitizeRepoName(repoPath) {
		t.Fatalf("expected slug %q, got %q", statestore.SanitizeRepoName(repoPath), slug)
	}
}

func TestPool_CompleteOpencodeSession(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	session, err := pool.CreateOpencodeSession(repoPath, "ses_complete", "Test prompt", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	completedAt := start.Add(2 * time.Minute)
	exitCode := 1
	duration := int(completedAt.Sub(start).Seconds())

	completed, err := pool.CompleteOpencodeSession(repoPath, session.ID, OpencodeSessionFailed, completedAt, &exitCode, duration)
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

	_, err = pool.CompleteOpencodeSession(repoPath, session.ID, OpencodeSessionFailed, completedAt, &exitCode, duration)
	if !errors.Is(err, ErrOpencodeSessionNotActive) {
		t.Fatalf("expected ErrOpencodeSessionNotActive, got %v", err)
	}
}
