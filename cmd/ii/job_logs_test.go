package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/opencode"
)

func TestJobLogSnapshotOrdersBySessionStart(t *testing.T) {
	stateDir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	opencodeStore, err := opencode.OpenWithOptions(opencode.Options{
		StateDir:    stateDir,
		StorageRoot: filepath.Join(home, ".local", "share", "opencode"),
		EventsDir:   filepath.Join(home, ".local", "share", "incrementum", "opencode", "events"),
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	repoPath := "/tmp/job-logs"
	startedAt := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	firstSession, err := opencodeStore.CreateSession(repoPath, "ses_implement", "Implement", startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	secondSession, err := opencodeStore.CreateSession(repoPath, "ses_review", "Review", startedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	eventsDir := filepath.Join(home, ".local", "share", "incrementum", "opencode", "events")
	writeSessionLog(t, eventsDir, firstSession.ID, "event: log\ndata: first\n\n")
	writeSessionLog(t, eventsDir, secondSession.ID, "event: log\ndata: second\n\n")

	stateStore := statestore.NewStore(stateDir)
	repoSlug, err := stateStore.GetOrCreateRepoName(repoPath)
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

	if err := stateStore.Update(func(st *statestore.State) error {
		st.Jobs[repoSlug+"/"+job.ID] = job
		return nil
	}); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	snapshot, err := jobLogSnapshot(opencodeStore, repoPath, job)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	expected := "==> implement (" + firstSession.ID + ")\nevent: log\ndata: first\n\n\n==> review (" + secondSession.ID + ")\nevent: log\ndata: second\n\n"
	if snapshot != expected {
		t.Fatalf("expected snapshot %q, got %q", expected, snapshot)
	}
}

func writeSessionLog(t *testing.T, eventsDir, sessionID, text string) {
	t.Helper()
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatalf("create events dir: %v", err)
	}
	path := filepath.Join(eventsDir, sessionID+".sse")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
