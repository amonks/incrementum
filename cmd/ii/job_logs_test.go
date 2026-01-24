package main

import (
	"encoding/json"
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

	storageRoot := filepath.Join(home, ".local", "share", "opencode", "storage")
	writeSessionLog(t, storageRoot, firstSession.ID, "first\n", startedAt)
	writeSessionLog(t, storageRoot, secondSession.ID, "second\n", startedAt.Add(2*time.Minute))

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

	expected := "==> implement (" + firstSession.ID + ")\nfirst\n\n==> review (" + secondSession.ID + ")\nsecond\n"
	if snapshot != expected {
		t.Fatalf("expected snapshot %q, got %q", expected, snapshot)
	}
}

func writeSessionLog(t *testing.T, storageRoot, sessionID, text string, createdAt time.Time) {
	t.Helper()
	messageDir := filepath.Join(storageRoot, "message", sessionID)
	partDir := filepath.Join(storageRoot, "part", "msg_"+sessionID)
	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		t.Fatalf("create message dir: %v", err)
	}
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatalf("create part dir: %v", err)
	}

	message := map[string]any{
		"id":        "msg_" + sessionID,
		"sessionID": sessionID,
		"role":      "assistant",
		"time": map[string]any{
			"created": createdAt.UnixMilli(),
		},
	}
	part := map[string]any{
		"id":        "prt_" + sessionID,
		"sessionID": sessionID,
		"messageID": "msg_" + sessionID,
		"type":      "text",
		"text":      text,
	}

	writeJSON(t, filepath.Join(messageDir, "msg_"+sessionID+".json"), message)
	writeJSON(t, filepath.Join(partDir, "prt_"+sessionID+".json"), part)
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
