package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/todo"
	"github.com/amonks/incrementum/workspace"
)

func TestManager_StartDone(t *testing.T) {
	repoPath := setupSessionRepo(t)
	homeDir := os.Getenv("HOME")
	stateDir := filepath.Join(homeDir, ".local", "state", "incr")
	workspacesDir := filepath.Join(homeDir, ".local", "share", "incr", "workspaces")

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Session todo", todo.CreateOptions{Priority: todo.PriorityMedium})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false})
	if err != nil {
		t.Fatalf("reopen todo store for session: %v", err)
	}
	defer store.Release()

	manager, err := Open(repoPath, OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false},
		Workspace: workspace.Options{
			StateDir:      stateDir,
			WorkspacesDir: workspacesDir,
		},
	})
	if err != nil {
		t.Fatalf("open session manager: %v", err)
	}
	defer manager.Close()

	if _, err := manager.pool.Acquire(repoPath, workspace.AcquireOptions{Rev: "@"}); err != nil {
		manager.Close()
		t.Fatalf("seed repo in pool: %v", err)
	}
	if err := manager.pool.ReleaseByName(repoPath, "ws-001"); err != nil {
		manager.Close()
		t.Fatalf("release seed workspace: %v", err)
	}

	startResult, err := manager.Start(created.ID, StartOptions{Rev: "@"})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if startResult.Session.Status != StatusActive {
		t.Fatalf("expected active status, got %q", startResult.Session.Status)
	}
	if startResult.WorkspacePath == "" {
		t.Fatal("expected workspace path")
	}
	if startResult.Session.Topic != "Session todo" {
		t.Fatalf("expected topic to default to title, got %q", startResult.Session.Topic)
	}

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false})
	if err != nil {
		t.Fatalf("reopen todo store: %v", err)
	}
	items, err := store.Show([]string{created.ID})
	if err != nil {
		store.Release()
		t.Fatalf("show todo: %v", err)
	}
	if items[0].Status != todo.StatusInProgress {
		store.Release()
		t.Fatalf("expected status in_progress, got %q", items[0].Status)
	}
	store.Release()

	final, err := manager.Done(created.ID, FinalizeOptions{})
	if err != nil {
		t.Fatalf("done session: %v", err)
	}
	if final.Status != StatusCompleted {
		t.Fatalf("expected completed session, got %q", final.Status)
	}

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false})
	if err != nil {
		t.Fatalf("reopen todo store: %v", err)
	}
	items, err = store.Show([]string{created.ID})
	if err != nil {
		store.Release()
		t.Fatalf("show todo: %v", err)
	}
	if items[0].Status != todo.StatusDone {
		store.Release()
		t.Fatalf("expected status done, got %q", items[0].Status)
	}
	store.Release()
}

func TestAgeUsesDurationSeconds(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)

	item := Session{
		Status:          StatusCompleted,
		StartedAt:       start,
		CompletedAt:     now,
		DurationSeconds: 90,
	}

	age := Age(item, now)
	if age != 90*time.Second {
		t.Fatalf("expected 90s duration, got %s", age)
	}
}

func setupSessionRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	return tmpDir
}
