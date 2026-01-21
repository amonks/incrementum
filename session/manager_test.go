package session

import (
	"encoding/json"
	"errors"
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
	stateDir := filepath.Join(homeDir, ".local", "state", "incrementum")
	workspacesDir := filepath.Join(homeDir, ".local", "share", "incrementum", "workspaces")

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

	if _, err := manager.pool.Acquire(repoPath, workspace.AcquireOptions{Rev: "@", Purpose: "seed"}); err != nil {
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

	statePath := filepath.Join(os.Getenv("HOME"), ".local", "state", "incrementum", "state.json")
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

func TestResolveActiveSessionMatchesSessionTodoPrefix(t *testing.T) {
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

	start := time.Now().UTC()
	_, err = manager.pool.CreateSession(repoPath, "abc12345", "ws-001", "Test topic", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	resolved, err := manager.ResolveActiveSession("abc", "")
	if err != nil {
		t.Fatalf("resolve active session: %v", err)
	}
	if resolved.TodoID != "abc12345" {
		t.Fatalf("expected todo ID abc12345, got %q", resolved.TodoID)
	}
}

func TestManagerListFiltersByStatus(t *testing.T) {
	repoPath := setupSessionRepo(t)

	manager, err := Open(repoPath, OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false},
	})
	if err != nil {
		t.Fatalf("open session manager: %v", err)
	}
	defer manager.Close()

	start := time.Now().UTC().Add(-time.Minute)
	activeSession, err := manager.pool.CreateSession(repoPath, "todo-1", "ws-001", "Active", start)
	if err != nil {
		t.Fatalf("create active session: %v", err)
	}

	completedSession, err := manager.pool.CreateSession(repoPath, "todo-2", "ws-002", "Completed", start.Add(10*time.Second))
	if err != nil {
		t.Fatalf("create completed session: %v", err)
	}
	if _, err := manager.pool.CompleteSession(repoPath, completedSession.ID, workspace.SessionCompleted, start.Add(20*time.Second), nil, 10); err != nil {
		t.Fatalf("complete session: %v", err)
	}

	list, err := manager.List(ListFilter{})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(list))
	}
	if list[0].ID != activeSession.ID {
		t.Fatalf("expected active session %q, got %q", activeSession.ID, list[0].ID)
	}

	completed := StatusCompleted
	list, err = manager.List(ListFilter{Status: &completed})
	if err != nil {
		t.Fatalf("list completed sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 completed session, got %d", len(list))
	}
	if list[0].Status != StatusCompleted {
		t.Fatalf("expected completed status, got %q", list[0].Status)
	}

	list, err = manager.List(ListFilter{IncludeAll: true})
	if err != nil {
		t.Fatalf("list all sessions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}

	invalid := Status("unknown")
	if _, err := manager.List(ListFilter{Status: &invalid}); err == nil || !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected invalid status error, got %v", err)
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
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incrementum"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incrementum", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	return tmpDir
}
