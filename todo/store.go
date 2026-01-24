package todo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/amonks/incrementum/internal/jj"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/workspace"
	"golang.org/x/term"
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
	client    *jj.Client
	readOnly  bool
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

	// Purpose describes why the store workspace is acquired.
	// If empty, a default purpose is used.
	Purpose string

	// ReadOnly opens the store without acquiring a workspace.
	// Read-only mode cannot create missing stores.
	ReadOnly bool
}

// Open opens the todo store for the repository at repoPath.
// If the incr/tasks bookmark doesn't exist and PromptToCreate is true,
// the user will be prompted to create it.
func Open(repoPath string, opts OpenOptions) (*Store, error) {
	usesStdioPrompter := opts.Prompter == nil
	if opts.Prompter == nil {
		opts.Prompter = StdioPrompter{}
	}

	purpose := internalstrings.NormalizeWhitespace(opts.Purpose)
	if purpose == "" {
		purpose = "todo store"
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
		if opts.ReadOnly {
			return nil, ErrNoTodoStore
		}
		if !opts.CreateIfMissing {
			return nil, ErrNoTodoStore
		}

		if opts.PromptToCreate {
			shouldPrompt := !usesStdioPrompter || term.IsTerminal(int(os.Stdin.Fd()))
			if shouldPrompt {
				confirmed, err := opts.Prompter.Confirm("No todo store found. Create one?")
				if err != nil {
					return nil, fmt.Errorf("prompt: %w", err)
				}
				if !confirmed {
					return nil, ErrNoTodoStore
				}
			}
		}
	}

	if opts.ReadOnly {
		return &Store{
			repoPath: repoPath,
			client:   client,
			prompter: opts.Prompter,
			readOnly: true,
		}, nil
	}

	pool, err := workspace.Open()
	if err != nil {
		return nil, fmt.Errorf("open workspace pool: %w", err)
	}

	// Acquire a workspace. If the bookmark doesn't exist yet, we'll create
	// the store in this workspace, then edit to it.
	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{Purpose: purpose})
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
		client:   client,
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

func storeFilePath(wsPath, filename string) string {
	return filepath.Join(wsPath, filename)
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

	return readJSONLFromReader[T](f)
}

func readJSONLFromReader[T any](reader io.Reader) ([]T, error) {
	var items []T
	scanner := bufio.NewScanner(reader)
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

func readJSONLStore[T any](store *Store, filename string) ([]T, error) {
	if store.readOnly {
		return readJSONLAtBookmark[T](store.client, store.repoPath, filename)
	}

	path := storeFilePath(store.wsPath, filename)
	var items []T
	err := withFileLock(path, func() error {
		var err error
		items, err = readJSONL[T](path)
		return err
	})
	return items, err
}

// readTodos reads all todos from the store.
func (s *Store) readTodos() ([]Todo, error) {
	return readJSONLStore[Todo](s, TodosFile)
}

func (s *Store) readTodosWithContext() ([]Todo, error) {
	todos, err := s.readTodos()
	if err != nil {
		return nil, fmt.Errorf("read todos: %w", err)
	}
	return todos, nil
}

// IDIndex returns an index of all todo IDs in the store.
func (s *Store) IDIndex() (IDIndex, error) {
	todos, err := s.readTodosWithContext()
	if err != nil {
		return IDIndex{}, err
	}
	return NewIDIndex(todos), nil
}

func writeJSONLStore[T any](store *Store, filename string, items []T) error {
	if err := store.ensureWritable(); err != nil {
		return err
	}

	path := storeFilePath(store.wsPath, filename)
	err := withFileLock(path, func() error {
		return writeJSONL(path, items)
	})
	if err != nil {
		return err
	}

	if store.snapshot == nil {
		return fmt.Errorf("snapshotter is not configured")
	}

	return store.snapshot.Snapshot(store.wsPath)
}

// writeTodos writes all todos to the store and runs jj snapshot.
func (s *Store) writeTodos(todos []Todo) error {
	return writeJSONLStore(s, TodosFile, todos)
}

// readDependencies reads all dependencies from the store.
func (s *Store) readDependencies() ([]Dependency, error) {
	return readJSONLStore[Dependency](s, DependenciesFile)
}

func (s *Store) readDependenciesWithContext() ([]Dependency, error) {
	deps, err := s.readDependencies()
	if err != nil {
		return nil, fmt.Errorf("read dependencies: %w", err)
	}
	return deps, nil
}

// writeDependencies writes all dependencies to the store and runs jj snapshot.
func (s *Store) writeDependencies(deps []Dependency) error {
	return writeJSONLStore(s, DependenciesFile, deps)
}

func (s *Store) resolveTodoIDs(ids []string) ([]string, error) {
	todos, err := s.readTodos()
	if err != nil {
		return nil, err
	}

	return resolveTodoIDsWithTodos(ids, todos)
}

func resolveTodoIDsWithTodos(ids []string, todos []Todo) ([]string, error) {
	if err := validateTodoIDs(ids); err != nil {
		return nil, err
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

func validateTodoIDs(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("no todo IDs provided")
	}
	return nil
}

func readJSONLAtBookmark[T any](client *jj.Client, repoPath, path string) ([]T, error) {
	if client == nil {
		return nil, fmt.Errorf("todo store is missing jj client")
	}
	output, err := client.FileShow(repoPath, BookmarkName, path)
	if errors.Is(err, jj.ErrFileNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return readJSONLFromReader[T](bytes.NewReader(output))
}

func (s *Store) ensureWritable() error {
	if s.readOnly {
		return ErrReadOnlyStore
	}
	if s.wsPath == "" {
		return fmt.Errorf("todo store workspace is not available")
	}
	return nil
}
