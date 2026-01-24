package job

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/opencode"
)

func TestLogSnapshotOrdersBySessionStart(t *testing.T) {
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

	repoPath := filepath.Join(t.TempDir(), "job-logs")
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

	job := Job{
		ID:        "job-logs",
		Repo:      "repo-slug",
		TodoID:    "todo-1",
		Stage:     StageImplementing,
		Status:    StatusActive,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
		OpencodeSessions: []OpencodeSession{
			{Purpose: "review", ID: secondSession.ID},
			{Purpose: "implement", ID: firstSession.ID},
		},
	}

	snapshot, err := LogSnapshot(opencodeStore, repoPath, job)
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
