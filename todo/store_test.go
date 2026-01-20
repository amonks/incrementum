package todo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/jj"
)

// mockPrompter implements Prompter for testing.
type mockPrompter struct {
	response bool
	err      error
	called   bool
}

func (m *mockPrompter) Confirm(message string) (bool, error) {
	m.called = true
	return m.response, m.err
}

// setupTestRepo creates a temporary jj repository for testing.
// It also sets HOME to a temp directory to prevent leaking state into
// ~/.local/state/incr and ~/.local/share/incr/workspaces.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	// Set HOME to a temp directory to isolate state/workspaces
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

func TestOpen_NoBookmark(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Try to open without CreateIfMissing - should fail
	_, err := Open(repoPath, OpenOptions{
		CreateIfMissing: false,
	})
	if !errors.Is(err, ErrNoTodoStore) {
		t.Errorf("expected ErrNoTodoStore, got %v", err)
	}
}

func TestOpen_CreateIfMissing(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Open with CreateIfMissing
	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Verify the bookmark was created
	client := jj.New()
	bookmarks, err := client.BookmarkList(repoPath)
	if err != nil {
		t.Fatalf("failed to list bookmarks: %v", err)
	}

	found := false
	for _, b := range bookmarks {
		if b == BookmarkName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("bookmark %q not found in %v", BookmarkName, bookmarks)
	}
}

func TestOpen_PromptToCreate_Confirmed(t *testing.T) {
	repoPath := setupTestRepo(t)

	prompter := &mockPrompter{response: true}

	store, err := Open(repoPath, OpenOptions{
		Prompter:        prompter,
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	if !prompter.called {
		t.Error("prompter was not called")
	}
}

func TestOpen_PromptToCreate_Declined(t *testing.T) {
	repoPath := setupTestRepo(t)

	prompter := &mockPrompter{response: false}

	_, err := Open(repoPath, OpenOptions{
		Prompter:        prompter,
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
	if !errors.Is(err, ErrNoTodoStore) {
		t.Errorf("expected ErrNoTodoStore, got %v", err)
	}

	if !prompter.called {
		t.Error("prompter was not called")
	}
}

func TestOpen_ExistingStore(t *testing.T) {
	repoPath := setupTestRepo(t)

	// Create the store first
	store1, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	store1.Release()

	// Open it again - should work without creating
	store2, err := Open(repoPath, OpenOptions{
		CreateIfMissing: false,
	})
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer store2.Release()
}

func TestStore_ReadWriteTodos(t *testing.T) {
	repoPath := setupTestRepo(t)

	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Initially empty
	todos, err := store.readTodos()
	if err != nil {
		t.Fatalf("failed to read todos: %v", err)
	}
	if len(todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(todos))
	}

	// Write some todos
	now := time.Now()
	testTodos := []Todo{
		{
			ID:        "abc12345",
			Title:     "Test todo 1",
			Status:    StatusOpen,
			Priority:  PriorityMedium,
			Type:      TypeTask,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "def67890",
			Title:     "Test todo 2",
			Status:    StatusInProgress,
			Priority:  PriorityHigh,
			Type:      TypeBug,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := store.writeTodos(testTodos); err != nil {
		t.Fatalf("failed to write todos: %v", err)
	}

	// Read them back
	todos, err = store.readTodos()
	if err != nil {
		t.Fatalf("failed to read todos: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}

	if todos[0].ID != "abc12345" {
		t.Errorf("expected ID 'abc12345', got %q", todos[0].ID)
	}
	if todos[1].Title != "Test todo 2" {
		t.Errorf("expected title 'Test todo 2', got %q", todos[1].Title)
	}
}

func TestStore_ReadWriteTodos_LongDescription(t *testing.T) {
	repoPath := setupTestRepo(t)

	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	longDescription := strings.Repeat("a", 100000)
	now := time.Now()
	input := []Todo{
		{
			ID:          "longdesc",
			Title:       "Long description todo",
			Description: longDescription,
			Status:      StatusOpen,
			Priority:    PriorityMedium,
			Type:        TypeTask,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	if err := store.writeTodos(input); err != nil {
		t.Fatalf("failed to write todos: %v", err)
	}

	output, err := store.readTodos()
	if err != nil {
		t.Fatalf("failed to read todos: %v", err)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(output))
	}
	if output[0].Description != longDescription {
		t.Errorf("description mismatch: expected %d bytes, got %d", len(longDescription), len(output[0].Description))
	}
}

func TestStore_ReadWriteDependencies(t *testing.T) {
	repoPath := setupTestRepo(t)

	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Initially empty
	deps, err := store.readDependencies()
	if err != nil {
		t.Fatalf("failed to read dependencies: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(deps))
	}

	// Write some dependencies
	now := time.Now()
	testDeps := []Dependency{
		{
			TodoID:      "abc12345",
			DependsOnID: "def67890",
			Type:        DepBlocks,
			CreatedAt:   now,
		},
	}

	if err := store.writeDependencies(testDeps); err != nil {
		t.Fatalf("failed to write dependencies: %v", err)
	}

	// Read them back
	deps, err = store.readDependencies()
	if err != nil {
		t.Fatalf("failed to read dependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}

	if deps[0].TodoID != "abc12345" {
		t.Errorf("expected TodoID 'abc12345', got %q", deps[0].TodoID)
	}
}

func TestStore_GetTodoByID(t *testing.T) {
	repoPath := setupTestRepo(t)

	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	now := time.Now()
	testTodos := []Todo{
		{
			ID:        "abc12345",
			Title:     "Test todo 1",
			Status:    StatusOpen,
			Priority:  PriorityMedium,
			Type:      TypeTask,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := store.writeTodos(testTodos); err != nil {
		t.Fatalf("failed to write todos: %v", err)
	}

	// Find existing todo
	todo, err := store.getTodoByID("abc12345")
	if err != nil {
		t.Fatalf("failed to get todo: %v", err)
	}
	if todo.Title != "Test todo 1" {
		t.Errorf("expected title 'Test todo 1', got %q", todo.Title)
	}

	// Try to find non-existent todo
	_, err = store.getTodoByID("nonexistent")
	if !errors.Is(err, ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

func TestReadJSONL_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	// Non-existent file should return empty slice
	items, err := readJSONL[Todo](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %d items", len(items))
	}

	// Empty file should return empty slice
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	items, err = readJSONL[Todo](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %d items", len(items))
	}
}

func TestWriteJSONL_Atomic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jsonl")

	now := time.Now()
	todos := []Todo{
		{
			ID:        "abc12345",
			Title:     "Test",
			Status:    StatusOpen,
			Priority:  PriorityMedium,
			Type:      TypeTask,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := writeJSONL(path, todos); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Verify no temp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file was not cleaned up")
	}

	// Verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(data) == 0 {
		t.Error("file is empty")
	}
}

// TestOpen_DoesNotModifyUserWorkingCopy verifies that opening/creating a store
// does not modify the user's working copy. This is a regression test for a bug
// where createTaskStore would run jj commands in the user's directory, causing
// the user's @ to temporarily move to the todo store change.
func TestOpen_DoesNotModifyUserWorkingCopy(t *testing.T) {
	repoPath := setupTestRepo(t)

	client := jj.New()

	// Record the user's current change ID before opening the store
	changeIDBefore, err := client.CurrentChangeID(repoPath)
	if err != nil {
		t.Fatalf("failed to get initial change ID: %v", err)
	}

	// Open the store (which will create the todo store since it doesn't exist)
	store, err := Open(repoPath, OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
	})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// The user's working copy should still be at the same change
	changeIDAfter, err := client.CurrentChangeID(repoPath)
	if err != nil {
		t.Fatalf("failed to get change ID after open: %v", err)
	}

	if changeIDBefore != changeIDAfter {
		t.Errorf("user's working copy was modified during store creation\n"+
			"before: %s\n"+
			"after:  %s\n"+
			"Store creation should operate in a background workspace, not the user's directory",
			changeIDBefore, changeIDAfter)
	}
}
