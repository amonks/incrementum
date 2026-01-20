package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Status represents the state of a workspace.
type Status string

const (
	// StatusAvailable indicates the workspace is free to be acquired.
	StatusAvailable Status = "available"

	// StatusAcquired indicates the workspace is currently in use.
	StatusAcquired Status = "acquired"

	// StatusStale indicates the workspace's lease has expired.
	StatusStale Status = "stale"
)

// repoInfo stores information about a tracked repository.
type repoInfo struct {
	SourcePath string `json:"source_path"`
}

// workspaceInfo stores information about a workspace.
type workspaceInfo struct {
	Name          string    `json:"name"`
	Repo          string    `json:"repo"`
	Path          string    `json:"path"`
	Status        Status    `json:"status"`
	AcquiredByPID int       `json:"acquired_by_pid,omitempty"`
	AcquiredAt    time.Time `json:"acquired_at,omitempty"`
	TTLSeconds    int       `json:"ttl_seconds,omitempty"`
	Provisioned   bool      `json:"provisioned"`
}

// state represents the persisted state of the workspace pool.
type state struct {
	Repos      map[string]repoInfo      `json:"repos"`
	Workspaces map[string]workspaceInfo `json:"workspaces"`
	Sessions   map[string]Session       `json:"sessions"`
}

// stateStore manages the state file with locking.
type stateStore struct {
	dir string
}

// newStateStore creates a new state store using the given directory.
func newStateStore(dir string) *stateStore {
	return &stateStore{dir: dir}
}

// statePath returns the path to the state file.
func (s *stateStore) statePath() string {
	return filepath.Join(s.dir, "state.json")
}

// lockPath returns the path to the lock file.
func (s *stateStore) lockPath() string {
	return filepath.Join(s.dir, "state.lock")
}

// load reads the state from disk. Returns an empty state if the file doesn't exist.
func (s *stateStore) load() (*state, error) {
	data, err := os.ReadFile(s.statePath())
	if os.IsNotExist(err) {
		return &state{
			Repos:      make(map[string]repoInfo),
			Workspaces: make(map[string]workspaceInfo),
			Sessions:   make(map[string]Session),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var st state
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	// Initialize maps if nil
	if st.Repos == nil {
		st.Repos = make(map[string]repoInfo)
	}
	if st.Workspaces == nil {
		st.Workspaces = make(map[string]workspaceInfo)
	}
	if st.Sessions == nil {
		st.Sessions = make(map[string]Session)
	}

	return &st, nil
}

// save writes the state to disk.
func (s *stateStore) save(st *state) error {
	// Ensure directory exists
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Write atomically via temp file
	tmpPath := s.statePath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, s.statePath()); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename state file: %w", err)
	}

	return nil
}

// update atomically reads, modifies, and writes the state with file locking.
func (s *stateStore) update(fn func(st *state) error) error {
	// Ensure directory exists
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	// Open lock file
	lockFile, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer lockFile.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Load current state
	st, err := s.load()
	if err != nil {
		return err
	}

	// Apply modifications
	if err := fn(st); err != nil {
		return err
	}

	// Save updated state
	return s.save(st)
}

// ErrRepoPathNotFound indicates a workspace is tracked but missing repo info.
var ErrRepoPathNotFound = fmt.Errorf("repo source path not found")

// repoPathForWorkspace returns the source repo path for a workspace path.
func (s *stateStore) repoPathForWorkspace(wsPath string) (string, bool, error) {
	st, err := s.load()
	if err != nil {
		return "", false, err
	}

	wsPath = filepath.Clean(wsPath)
	for _, ws := range st.Workspaces {
		if filepath.Clean(ws.Path) != wsPath {
			continue
		}
		repo, ok := st.Repos[ws.Repo]
		if !ok || repo.SourcePath == "" {
			return "", true, ErrRepoPathNotFound
		}
		return repo.SourcePath, true, nil
	}

	return "", false, nil
}

// getOrCreateRepoName returns the repo name for the given source path,
// creating a new entry if needed. Handles collisions by appending suffixes.
func (s *stateStore) getOrCreateRepoName(sourcePath string) (string, error) {
	var result string

	err := s.update(func(st *state) error {
		// Check if this path already has a name
		for name, info := range st.Repos {
			if info.SourcePath == sourcePath {
				result = name
				return nil
			}
		}

		// Generate a new name
		baseName := sanitizeRepoName(sourcePath)
		name := baseName

		// Handle collisions
		suffix := 2
		for {
			if _, exists := st.Repos[name]; !exists {
				break
			}
			name = fmt.Sprintf("%s-%d", baseName, suffix)
			suffix++
		}

		st.Repos[name] = repoInfo{SourcePath: sourcePath}
		result = name
		return nil
	})

	return result, err
}

// sanitizeRepoName converts a file path to a safe repo name.
func sanitizeRepoName(path string) string {
	// Expand ~ if present
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}

	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Convert to lowercase
	path = strings.ToLower(path)

	// Replace path separators and spaces with hyphens
	path = strings.ReplaceAll(path, "/", "-")
	path = strings.ReplaceAll(path, " ", "-")

	// Remove any characters that aren't alphanumeric or hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	path = reg.ReplaceAllString(path, "")

	// Collapse multiple hyphens
	reg = regexp.MustCompile(`-+`)
	path = reg.ReplaceAllString(path, "-")

	// Trim leading/trailing hyphens
	path = strings.Trim(path, "-")

	return path
}
