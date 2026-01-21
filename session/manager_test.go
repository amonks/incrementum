package session

import (
	"encoding/json"
	"fmt"
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

	final, err := manager.Done(created.ID, FinalizeOptions{WorkspacePath: startResult.WorkspacePath})
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

func TestManager_RunReleasesWorkspaceOnSessionUpdateError(t *testing.T) {
	repoPath := setupSessionRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Session run", todo.CreateOptions{Priority: todo.PriorityMedium})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	manager, err := Open(repoPath, OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false},
	})
	if err != nil {
		t.Fatalf("open session manager: %v", err)
	}
	defer manager.Close()

	type rawState struct {
		Repos      map[string]json.RawMessage `json:"repos"`
		Workspaces map[string]json.RawMessage `json:"workspaces"`
		Sessions   map[string]json.RawMessage `json:"sessions"`
	}

	statePath := filepath.Join(os.Getenv("HOME"), ".local", "state", "incr", "state.json")
	removeErr := make(chan error, 1)
	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			data, err := os.ReadFile(statePath)
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			var st rawState
			if err := json.Unmarshal(data, &st); err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if len(st.Sessions) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			removed := false
			for key, raw := range st.Sessions {
				var entry struct {
					TodoID string `json:"todo_id"`
				}
				if err := json.Unmarshal(raw, &entry); err != nil {
					continue
				}
				if entry.TodoID == created.ID {
					delete(st.Sessions, key)
					removed = true
				}
			}
			if removed {
				updated, err := json.MarshalIndent(st, "", "  ")
				if err != nil {
					removeErr <- err
					return
				}
				if err := os.WriteFile(statePath, updated, 0644); err != nil {
					removeErr <- err
					return
				}
				removeErr <- nil
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		removeErr <- fmt.Errorf("timed out waiting for session state")
	}()

	_, err = manager.Run(created.ID, RunOptions{Command: []string{"sh", "-c", "sleep 0.2"}})
	if err == nil {
		t.Fatal("expected error from run")
	}

	select {
	case err := <-removeErr:
		if err != nil {
			t.Fatalf("remove session entry: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session removal")
	}

	infos, err := manager.pool.List(repoPath)
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	acquired := 0
	for _, info := range infos {
		if info.Status == workspace.StatusAcquired {
			acquired++
		}
	}
	if acquired != 1 {
		t.Fatalf("expected 1 acquired workspace, got %d", acquired)
	}
}

func TestResolveActiveSessionRequiresWorkspace(t *testing.T) {
	repoPath := setupSessionRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	store.Release()

	manager, err := Open(repoPath, OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false},
	})
	if err != nil {
		t.Fatalf("open session manager: %v", err)
	}
	defer manager.Close()

	_, err = manager.ResolveActiveSession("", repoPath)
	if err == nil {
		t.Fatal("expected error for non-workspace path")
	}
	if err.Error() != "todo id required when not in a workspace" {
		t.Fatalf("unexpected error: %v", err)
	}
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
