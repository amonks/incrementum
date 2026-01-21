package todo

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/workspace"
)

const (
	// BookmarkName is the jj bookmark used to identify the todo store change.
	BookmarkName = "incr/tasks"

	// TodosFile is the name of the JSONL file containing todos.
	TodosFile = "todos.jsonl"

	// DependenciesFile is the name of the JSONL file containing dependencies.
	DependenciesFile = "dependencies.jsonl"

	maxJSONLineBytes = 1024 * 1024
)

// Store provides access to the todo data for a jujutsu repository.
// It manages workspace acquisition and file locking for concurrent access.
type Store struct {
	repoPath  string
	wsPath    string
	pool      *workspace.Pool
	snapshot  Snapshotter
	prompter  Prompter
	wsRelease func() error
}

// Snapshotter records workspace changes.
type Snapshotter interface {
	Snapshot(workspacePath string) error
}

// Prompter is used to ask the user for confirmation.
type Prompter interface {
	// Confirm asks the user a yes/no question and returns true if they say yes.
	Confirm(message string) (bool, error)
}

// StdioPrompter implements Prompter using stdin/stdout.
type StdioPrompter struct{}

// Confirm asks the user a yes/no question via stdin/stdout.
func (p StdioPrompter) Confirm(message string) (bool, error) {
	fmt.Printf("%s [y/n]: ", message)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false, err
	}
	return response == "y" || response == "Y" || response == "yes" || response == "Yes", nil
}

// OpenOptions configures how the store is opened.
type OpenOptions struct {
	// Prompter is used for user confirmation. If nil, StdioPrompter is used.
	Prompter Prompter

	// CreateIfMissing creates the todo store if it doesn't exist.
	// If false and the store doesn't exist, ErrNoTodoStore is returned.
	CreateIfMissing bool

	// PromptToCreate prompts the user before creating a new store.
	// Only used when CreateIfMissing is true.
	PromptToCreate bool
}

// Open opens the todo store for the repository at repoPath.
// If the incr/tasks bookmark doesn't exist and PromptToCreate is true,
// the user will be prompted to create it.
func Open(repoPath string, opts OpenOptions) (*Store, error) {
	if opts.Prompter == nil {
		opts.Prompter = StdioPrompter{}
	}

	pool, err := workspace.Open()
	if err != nil {
		return nil, fmt.Errorf("open workspace pool: %w", err)
	}

	client := jj.New()

	// Update stale working copy before listing bookmarks
	// This can happen if another workspace has modified the repo
	_ = client.WorkspaceUpdateStale(repoPath)

	// Check if the bookmark exists
	bookmarks, err := client.BookmarkList(repoPath)
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}

	hasBookmark := false
	for _, b := range bookmarks {
		if b == BookmarkName {
			hasBookmark = true
			break
		}
	}

	if !hasBookmark {
		if !opts.CreateIfMissing {
			return nil, ErrNoTodoStore
		}

		if opts.PromptToCreate {
			confirmed, err := opts.Prompter.Confirm("No todo store found. Create one?")
			if err != nil {
				return nil, fmt.Errorf("prompt: %w", err)
			}
			if !confirmed {
				return nil, ErrNoTodoStore
			}
		}
	}

	// Acquire a workspace. If the bookmark doesn't exist yet, we'll create
	// the store in this workspace, then edit to it.
	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{Purpose: "todo store"})
	if err != nil {
		return nil, fmt.Errorf("acquire workspace: %w", err)
	}

	// If the bookmark doesn't exist, create the store in our workspace
	if !hasBookmark {
		if err := createTodoStore(client, wsPath); err != nil {
			pool.Release(wsPath)
			return nil, fmt.Errorf("create todo store: %w", err)
		}
	}

	// Edit to the bookmark
	if err := client.Edit(wsPath, BookmarkName); err != nil {
		pool.Release(wsPath)
		return nil, fmt.Errorf("edit to todo store: %w", err)
	}

	// Update stale working copy if needed (this can happen if the bookmark
	// was moved after the workspace was last used)
	if err := client.WorkspaceUpdateStale(wsPath); err != nil {
		// Ignore errors - the workspace might not be stale
		// The error message is not helpful for detecting this case
	}

	return &Store{
		repoPath: repoPath,
		wsPath:   wsPath,
		pool:     pool,
		snapshot: client,
		prompter: opts.Prompter,
		wsRelease: func() error {
			return pool.Release(wsPath)
		},
	}, nil
}

// Release releases the workspace back to the pool.
// This should be called when done using the store.
func (s *Store) Release() error {
	if s.wsRelease != nil {
		return s.wsRelease()
	}
	return nil
}

// createTodoStore creates the orphan change and bookmark for the todo store.
// wsPath must be an already-acquired workspace.
func createTodoStore(client *jj.Client, wsPath string) error {
	// Create a new change at root() in the workspace.
	// This moves the workspace's @ to the new orphan change.
	changeID, err := client.NewChange(wsPath, "root()")
	if err != nil {
		return fmt.Errorf("create orphan change: %w", err)
	}

	// Set the description
	if err := client.Describe(wsPath, "ii todo store - do not edit directly"); err != nil {
		return fmt.Errorf("describe change: %w", err)
	}

	// Create the bookmark pointing to the new change.
	// This bookmark is visible repo-wide, not just in this workspace.
	if err := client.BookmarkCreate(wsPath, BookmarkName, changeID); err != nil {
		return fmt.Errorf("create bookmark: %w", err)
	}

	return nil
}

// todosPath returns the path to the todos.jsonl file.
func (s *Store) todosPath() string {
	return filepath.Join(s.wsPath, TodosFile)
}

// dependenciesPath returns the path to the dependencies.jsonl file.
func (s *Store) dependenciesPath() string {
	return filepath.Join(s.wsPath, DependenciesFile)
}

// withFileLock executes fn while holding an exclusive lock on the file at path.
// Creates the file if it doesn't exist.
func withFileLock(path string, fn func() error) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	// Open or create the file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open file for locking: %w", err)
	}
	defer f.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

// readJSONL reads all JSON objects from a JSONL file into a slice.
func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var items []T
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxJSONLineBytes)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, fmt.Errorf("parse line %d: %w", lineNum, err)
		}
		items = append(items, item)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}

	return items, nil
}

// writeJSONL writes a slice of items to a JSONL file, overwriting any existing content.
func writeJSONL[T any](path string, items []T) error {
	// Write to temp file first
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	encoder := json.NewEncoder(f)
	for i, item := range items {
		if err := encoder.Encode(item); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode item %d: %w", i, err)
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// readTodos reads all todos from the store.
func (s *Store) readTodos() ([]Todo, error) {
	var todos []Todo
	err := withFileLock(s.todosPath(), func() error {
		var err error
		todos, err = readJSONL[Todo](s.todosPath())
		return err
	})
	return todos, err
}

// IDIndex returns an index of all todo IDs in the store.
func (s *Store) IDIndex() (IDIndex, error) {
	todos, err := s.readTodos()
	if err != nil {
		return IDIndex{}, fmt.Errorf("read todos: %w", err)
	}
	return NewIDIndex(todos), nil
}

// writeTodos writes all todos to the store and runs jj snapshot.
func (s *Store) writeTodos(todos []Todo) error {
	err := withFileLock(s.todosPath(), func() error {
		return writeJSONL(s.todosPath(), todos)
	})
	if err != nil {
		return err
	}

	if s.snapshot == nil {
		return fmt.Errorf("snapshotter is not configured")
	}

	return s.snapshot.Snapshot(s.wsPath)
}

// readDependencies reads all dependencies from the store.
func (s *Store) readDependencies() ([]Dependency, error) {
	var deps []Dependency
	err := withFileLock(s.dependenciesPath(), func() error {
		var err error
		deps, err = readJSONL[Dependency](s.dependenciesPath())
		return err
	})
	return deps, err
}

// writeDependencies writes all dependencies to the store and runs jj snapshot.
func (s *Store) writeDependencies(deps []Dependency) error {
	err := withFileLock(s.dependenciesPath(), func() error {
		return writeJSONL(s.dependenciesPath(), deps)
	})
	if err != nil {
		return err
	}

	if s.snapshot == nil {
		return fmt.Errorf("snapshotter is not configured")
	}

	return s.snapshot.Snapshot(s.wsPath)
}

// getTodoByID finds a todo by its ID.
func (s *Store) getTodoByID(id string) (*Todo, error) {
	resolved, err := s.resolveTodoIDs([]string{id})
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, ErrTodoNotFound
	}

	todos, err := s.readTodos()
	if err != nil {
		return nil, err
	}

	for i := range todos {
		if todos[i].ID == resolved[0] {
			return &todos[i], nil
		}
	}

	return nil, ErrTodoNotFound
}

func (s *Store) resolveTodoIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no todo IDs provided")
	}

	todos, err := s.readTodos()
	if err != nil {
		return nil, err
	}

	return resolveTodoIDsWithTodos(ids, todos)
}

func resolveTodoIDsWithTodos(ids []string, todos []Todo) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no todo IDs provided")
	}

	index := NewIDIndex(todos)
	resolved := make([]string, 0, len(ids))
	for _, id := range ids {
		resolvedID, err := index.Resolve(id)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedID)
	}

	return resolved, nil
}
