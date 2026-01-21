package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/internal/jj"
)

// DefaultTTL is the default lease duration for acquired workspaces.
const DefaultTTL = time.Hour

// Pool manages a pool of jujutsu workspaces.
//
// A Pool maintains workspaces in a shared location and tracks which workspaces
// are currently acquired. Multiple processes can safely use the same Pool
// concurrently through file-based locking.
type Pool struct {
	stateStore    *stateStore
	workspacesDir string
	jj            *jj.Client
}

// Options configures a workspace pool.
type Options struct {
	// StateDir is the directory where pool state is stored.
	// Defaults to ~/.local/state/incrementum if empty.
	StateDir string

	// WorkspacesDir is the directory where workspaces are created.
	// Defaults to ~/.local/share/incrementum/workspaces if empty.
	WorkspacesDir string
}

// Open creates a new Pool with default options.
// State is stored in ~/.local/state/incrementum and workspaces in
// ~/.local/share/incrementum/workspaces.
func Open() (*Pool, error) {
	return OpenWithOptions(Options{})
}

// OpenWithOptions creates a new Pool with custom options.
func OpenWithOptions(opts Options) (*Pool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	stateDir := opts.StateDir
	if stateDir == "" {
		stateDir = filepath.Join(home, ".local", "state", "incrementum")
	}

	workspacesDir := opts.WorkspacesDir
	if workspacesDir == "" {
		workspacesDir = filepath.Join(home, ".local", "share", "incrementum", "workspaces")
	}

	return &Pool{
		stateStore:    newStateStore(stateDir),
		workspacesDir: workspacesDir,
		jj:            jj.New(),
	}, nil
}

// AcquireOptions configures a workspace acquire operation.
type AcquireOptions struct {
	// Rev is the jj revision to check out. Defaults to "@" if empty.
	Rev string

	// TTL is how long the lease is valid before expiring.
	// Defaults to DefaultTTL (1 hour) if zero.
	TTL time.Duration

	// Purpose describes why the workspace is being acquired.
	// It must be a single-line string.
	Purpose string
}

// Acquire obtains a workspace from the pool for the given repository.
//
// If an available workspace exists, it will be reused. Otherwise, a new
// workspace is created. The workspace is checked out to the specified
// revision (or @ by default).
//
// The returned path is the root directory of the acquired workspace.
// Call Release when done to return the workspace to the pool.
//
// If the repository contains a .incr.toml configuration file, the on-create
// hooks run on every acquire.

func (p *Pool) Acquire(repoPath string, opts AcquireOptions) (string, error) {
	// Apply defaults
	if opts.Rev == "" {
		opts.Rev = "@"
	}
	if opts.TTL == 0 {
		opts.TTL = DefaultTTL
	}
	if strings.TrimSpace(opts.Purpose) == "" {
		return "", fmt.Errorf("purpose is required")
	}
	if strings.ContainsAny(opts.Purpose, "\r\n") {
		return "", fmt.Errorf("purpose must be a single line")
	}

	// Get the repo name (creates entry if needed)
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return "", fmt.Errorf("get repo name: %w", err)
	}

	var wsPath string
	var wsName string
	var needsCreate bool
	var needsProvision bool

	// Find or create a workspace
	err = p.stateStore.update(func(state *state) error {
		// First, expire stale workspaces
		now := time.Now()
		for key, ws := range state.Workspaces {
			if ws.Repo == repoName && ws.Status == StatusAcquired {
				expiry := ws.AcquiredAt.Add(time.Duration(ws.TTLSeconds) * time.Second)
				if now.After(expiry) {
					ws.Status = StatusAvailable
					ws.AcquiredByPID = 0
					ws.AcquiredAt = time.Time{}
					ws.TTLSeconds = 0
					state.Workspaces[key] = ws
				}
			}
		}

		// Find an available workspace
		for key, ws := range state.Workspaces {
			if ws.Repo == repoName && ws.Status == StatusAvailable {
				wsPath = ws.Path
				wsName = ws.Name
				needsProvision = !ws.Provisioned

				// Acquire it
				ws.Status = StatusAcquired
				ws.Purpose = opts.Purpose
				ws.AcquiredByPID = os.Getpid()
				ws.AcquiredAt = now
				ws.TTLSeconds = int(opts.TTL.Seconds())
				state.Workspaces[key] = ws
				return nil
			}
		}

		// No available workspace - create a new one
		wsName = p.nextWorkspaceName(state, repoName)
		wsPath = filepath.Join(p.workspacesDir, repoName, wsName)
		needsCreate = true
		needsProvision = true

		wsKey := repoName + "/" + wsName
		state.Workspaces[wsKey] = workspaceInfo{
			Name:          wsName,
			Repo:          repoName,
			Path:          wsPath,
			Purpose:       opts.Purpose,
			Status:        StatusAcquired,
			AcquiredByPID: os.Getpid(),
			AcquiredAt:    now,
			TTLSeconds:    int(opts.TTL.Seconds()),
			Provisioned:   false,
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	// Create the workspace directory if needed
	if needsCreate {
		if err := os.MkdirAll(filepath.Dir(wsPath), 0755); err != nil {
			return "", fmt.Errorf("create workspace parent dir: %w", err)
		}

		if err := p.jj.WorkspaceAdd(repoPath, wsName, wsPath); err != nil {
			// Clean up state on failure
			p.stateStore.update(func(state *state) error {
				delete(state.Workspaces, repoName+"/"+wsName)
				return nil
			})
			return "", fmt.Errorf("jj workspace add: %w", err)
		}
	}

	// Edit to the specified revision
	if err := p.jj.Edit(wsPath, opts.Rev); err != nil {
		return "", fmt.Errorf("jj edit: %w", err)
	}

	if err := p.ensureReleaseChange(wsPath, opts.Rev); err != nil {
		p.Release(wsPath)
		return "", err
	}

	// Load config and run hooks
	cfg, err := config.Load(repoPath)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	// Run on-create script for every acquire
	if err := config.RunScript(wsPath, cfg.Workspace.OnCreate); err != nil {
		p.Release(wsPath)
		return "", fmt.Errorf("on-create script: %w", err)
	}

	// Mark as provisioned if needed
	if needsProvision {
		p.stateStore.update(func(state *state) error {
			wsKey := repoName + "/" + wsName
			if ws, ok := state.Workspaces[wsKey]; ok {
				ws.Provisioned = true
				state.Workspaces[wsKey] = ws
			}
			return nil
		})
	}

	return wsPath, nil
}

// Release returns a workspace to the pool, making it available for reuse.
//
// After releasing, the workspace path should no longer be used. The workspace
// directory remains on disk and may be acquired again later.
func (p *Pool) Release(wsPath string) error {
	return p.releaseToAvailable(wsPath)
}

func (p *Pool) releaseToAvailable(wsPath string) error {
	if _, err := p.jj.NewChange(wsPath, "root()"); err != nil {
		return fmt.Errorf("jj new root(): %w", err)
	}

	return p.stateStore.update(func(state *state) error {
		for key, ws := range state.Workspaces {
			if ws.Path == wsPath {
				ws.Status = StatusAvailable
				ws.AcquiredByPID = 0
				ws.AcquiredAt = time.Time{}
				ws.TTLSeconds = 0
				state.Workspaces[key] = ws
				return nil
			}
		}
		return fmt.Errorf("workspace not found: %s", wsPath)
	})
}

func (p *Pool) ensureReleaseChange(wsPath, rev string) error {
	if _, err := p.jj.NewChange(wsPath, "root()"); err != nil {
		return fmt.Errorf("jj new root(): %w", err)
	}

	if err := p.jj.Edit(wsPath, rev); err != nil {
		return fmt.Errorf("jj edit: %w", err)
	}

	return nil
}

// ReleaseByName returns a workspace to the pool by name.
func (p *Pool) ReleaseByName(repoPath, wsName string) error {
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.load()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	key := repoName + "/" + wsName
	ws, ok := st.Workspaces[key]
	if !ok {
		return fmt.Errorf("workspace not found: %s", wsName)
	}

	return p.releaseToAvailable(ws.Path)
}

// Renew extends the TTL for an acquired workspace.
//
// Call this periodically to prevent a long-running lease from expiring.
// The TTL is reset to its original duration from the current time.
func (p *Pool) Renew(wsPath string) error {
	return p.stateStore.update(func(state *state) error {
		for key, ws := range state.Workspaces {
			if ws.Path == wsPath {
				if ws.Status != StatusAcquired {
					return fmt.Errorf("workspace is not acquired")
				}
				ws.AcquiredAt = time.Now()
				state.Workspaces[key] = ws
				return nil
			}
		}
		return fmt.Errorf("workspace not found: %s", wsPath)
	})
}

// RenewByName extends the TTL for an acquired workspace by name.
func (p *Pool) RenewByName(repoPath, wsName string) error {
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return fmt.Errorf("get repo name: %w", err)
	}

	return p.stateStore.update(func(state *state) error {
		key := repoName + "/" + wsName
		ws, ok := state.Workspaces[key]
		if !ok {
			return fmt.Errorf("workspace not found: %s", wsName)
		}
		if ws.Status != StatusAcquired {
			return fmt.Errorf("workspace is not acquired")
		}
		ws.AcquiredAt = time.Now()
		state.Workspaces[key] = ws
		return nil
	})
}

// Info contains information about a workspace.
type Info struct {
	// Name is the workspace identifier (e.g., "ws-001").
	Name string

	// Path is the absolute path to the workspace directory.
	Path string

	// Purpose describes why the workspace was acquired.
	Purpose string

	// Status indicates whether the workspace is available, acquired, or stale.
	Status Status

	// AcquiredByPID is the process ID that acquired this workspace.
	// Zero if not acquired.
	AcquiredByPID int

	// AcquiredAt is when the workspace was acquired.
	// Zero if not acquired.
	AcquiredAt time.Time

	// TTLRemaining is the time until the lease expires.
	// Zero if not acquired or already expired.
	TTLRemaining time.Duration
}

// List returns information about all workspaces for the given repository.
//
// The returned slice includes both available and acquired workspaces.

func (p *Pool) List(repoPath string) ([]Info, error) {
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	var items []Info
	now := time.Now()

	for _, ws := range st.Workspaces {
		if ws.Repo != repoName {
			continue
		}

		item := Info{
			Name:          ws.Name,
			Path:          ws.Path,
			Purpose:       ws.Purpose,
			Status:        ws.Status,
			AcquiredByPID: ws.AcquiredByPID,
			AcquiredAt:    ws.AcquiredAt,
		}

		// Calculate TTL remaining if acquired
		if ws.Status == StatusAcquired {
			expiry := ws.AcquiredAt.Add(time.Duration(ws.TTLSeconds) * time.Second)
			if now.After(expiry) {
				item.Status = StatusStale
			} else {
				item.TTLRemaining = expiry.Sub(now)
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// RepoRoot returns the jj repository root for the given path.
//
// This can be used to find the repository root before calling Acquire.
// Returns an error if the path is not inside a jj repository.
func RepoRoot(path string) (string, error) {
	client := jj.New()
	return client.WorkspaceRoot(path)
}

// ErrWorkspaceRootNotFound indicates a path is not in a jj workspace.
var ErrWorkspaceRootNotFound = fmt.Errorf("workspace root not found")

// RepoRootFromPath returns the source repo root for a workspace or repo path.
// If the path is a workspace, it resolves to the original repo using state.
func RepoRootFromPath(path string) (string, error) {
	return repoRootFromPathWithOptions(path, Options{})
}

// RepoRootFromPathWithOptions is like RepoRootFromPath with custom options.
func RepoRootFromPathWithOptions(path string, opts Options) (string, error) {
	return repoRootFromPathWithOptions(path, opts)
}

// WorkspaceNameForPath returns the workspace name for a workspace path.
// Returns ErrWorkspaceRootNotFound if the path is not a workspace.
func (p *Pool) WorkspaceNameForPath(path string) (string, error) {
	root, err := RepoRoot(path)
	if err != nil {
		return "", ErrWorkspaceRootNotFound
	}

	st, err := p.stateStore.load()
	if err != nil {
		return "", fmt.Errorf("load state: %w", err)
	}

	root = filepath.Clean(root)
	for _, ws := range st.Workspaces {
		if filepath.Clean(ws.Path) == root {
			return ws.Name, nil
		}
	}

	return "", ErrRepoPathNotFound
}

func repoRootFromPathWithOptions(path string, opts Options) (string, error) {
	root, err := RepoRoot(path)
	if err != nil {
		return "", ErrWorkspaceRootNotFound
	}

	pool, err := OpenWithOptions(opts)
	if err != nil {
		return "", fmt.Errorf("open workspace pool: %w", err)
	}

	repoPath, found, err := pool.stateStore.repoPathForWorkspace(root)
	if err != nil {
		return "", err
	}

	if found {
		if repoPath == "" {
			return "", ErrRepoPathNotFound
		}
		return repoPath, nil
	}

	rel, err := filepath.Rel(pool.workspacesDir, root)
	if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return "", ErrRepoPathNotFound
	}

	return root, nil
}

// nextWorkspaceName returns the next sequential workspace name for the repo.
func (p *Pool) nextWorkspaceName(st *state, repoName string) string {
	maxNum := 0
	for _, ws := range st.Workspaces {
		if ws.Repo == repoName {
			var num int
			if _, err := fmt.Sscanf(ws.Name, "ws-%d", &num); err == nil {
				if num > maxNum {
					maxNum = num
				}
			}
		}
	}
	return fmt.Sprintf("ws-%03d", maxNum+1)
}

// DestroyAll removes all workspaces for the given repository.
//
// This deletes both the state entries and the workspace directories on disk.
// It also runs "jj workspace forget" to unregister each workspace from the
// source repository.
func (p *Pool) DestroyAll(repoPath string) error {
	repoName, err := p.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		return fmt.Errorf("get repo name: %w", err)
	}

	var workspaces []workspaceInfo
	var repoSourcePath string

	// Collect workspaces to destroy and get the source repo path
	err = p.stateStore.update(func(state *state) error {
		// Get the source repo path
		if repo, ok := state.Repos[repoName]; ok {
			repoSourcePath = repo.SourcePath
		}

		for key, ws := range state.Workspaces {
			if ws.Repo == repoName {
				workspaces = append(workspaces, ws)
				delete(state.Workspaces, key)
			}
		}

		for key, session := range state.Sessions {
			if session.Repo == repoName {
				delete(state.Sessions, key)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Forget workspaces from jj and delete directories
	var errs []error
	for _, ws := range workspaces {
		// Try to forget from jj (may fail if source repo is gone)
		if repoSourcePath != "" {
			if err := p.jj.WorkspaceForget(repoSourcePath, ws.Name); err != nil {
				// Non-fatal - the workspace might already be forgotten or the repo gone
				errs = append(errs, fmt.Errorf("forget workspace %s: %w", ws.Name, err))
			}
		}

		// Delete the workspace directory
		if err := os.RemoveAll(ws.Path); err != nil {
			errs = append(errs, fmt.Errorf("remove workspace %s: %w", ws.Path, err))
		}
	}

	// Also try to remove the repo's workspace directory if empty
	repoWorkspacesDir := filepath.Join(p.workspacesDir, repoName)
	os.Remove(repoWorkspacesDir) // Ignore error - may not be empty or exist

	if len(errs) > 0 {
		// Return first error but log intent that some cleanup failed
		return errs[0]
	}

	return nil
}
