package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/workspace"
)

func TestJobLogSnapshotOrdersBySessionStart(t *testing.T) {
	stateDir := t.TempDir()
	pool, err := workspace.OpenWithOptions(workspace.Options{
		StateDir:      stateDir,
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/job-logs"
	logDir := t.TempDir()
	logOne := filepath.Join(logDir, "one.log")
	logTwo := filepath.Join(logDir, "two.log")
	if err := os.WriteFile(logOne, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write log one: %v", err)
	}
	if err := os.WriteFile(logTwo, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write log two: %v", err)
	}

	startedAt := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	firstSession, err := pool.CreateOpencodeSession(repoPath, "Implement", logOne, startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	secondSession, err := pool.CreateOpencodeSession(repoPath, "Review", logTwo, startedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	store := statestore.NewStore(stateDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	job := jobpkg.Job{
		ID:        "job-logs",
		Repo:      repoSlug,
		TodoID:    "todo-1",
		Stage:     jobpkg.StageImplementing,
		Status:    jobpkg.StatusActive,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
		OpencodeSessions: []jobpkg.OpencodeSession{
			{Purpose: "review", ID: secondSession.ID},
			{Purpose: "implement", ID: firstSession.ID},
		},
	}

	if err := store.Update(func(st *statestore.State) error {
		st.Jobs[repoSlug+"/"+job.ID] = job
		return nil
	}); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	snapshot, err := jobLogSnapshot(pool, repoPath, job)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	expected := "==> implement (" + firstSession.ID + ")\nfirst\n\n==> review (" + secondSession.ID + ")\nsecond\n"
	if snapshot != expected {
		t.Fatalf("expected snapshot %q, got %q", expected, snapshot)
	}
}
