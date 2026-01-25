package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/internal/paths"
	statestore "github.com/amonks/incrementum/internal/state"
)

// Pool manages a pool of jujutsu workspaces.
//
// A Pool maintains workspaces in a shared location and tracks which workspaces
// are currently acquired. Multiple processes can safely use the same Pool
// concurrently through file-based locking.
type Pool struct {
	stateStore    *statestore.Store
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
	stateDir := opts.StateDir
	if stateDir == "" {
		var err error
		stateDir, err = paths.DefaultStateDir()
		if err != nil {
			return nil, err
		}
	}

	workspacesDir := opts.WorkspacesDir
	if workspacesDir == "" {
		var err error
		workspacesDir, err = paths.DefaultWorkspacesDir()
		if err != nil {
			return nil, err
		}
	}

	return &Pool{
		stateStore:    statestore.NewStore(stateDir),
		workspacesDir: workspacesDir,
		jj:            jj.New(),
	}, nil
}

// RepoSlug returns the repo slug used for state storage.
func (p *Pool) RepoSlug(repoPath string) (string, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return "", fmt.Errorf("get repo name: %w", err)
	}
	return repoName, nil
}

// AcquireOptions configures a workspace acquire operation.
type AcquireOptions struct {
	// Rev is the jj revision to check out. Defaults to "@" if empty.
	Rev string

	// Purpose describes why the workspace is being acquired.
	// It must be a single-line string.
	Purpose string

	// NewChangeMessage is an optional description to apply when a new change
	// is created because the requested revision is immutable.
	NewChangeMessage string
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
// If the repository contains a incrementum.toml configuration file, the on-create
// hooks run on every acquire.

func (p *Pool) Acquire(repoPath string, opts AcquireOptions) (string, error) {
	// Apply defaults
	if opts.Rev == "" {
		opts.Rev = "@"
	}
	if strings.TrimSpace(opts.Purpose) == "" {
		return "", fmt.Errorf("purpose is required")
	}
	if strings.ContainsAny(opts.Purpose, "\r\n") {
		return "", fmt.Errorf("purpose must be a single line")
	}

	// Get the repo name (creates entry if needed)
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return "", fmt.Errorf("get repo name: %w", err)
	}

	var wsPath string
	var wsName string
	var needsCreate bool
	var needsProvision bool

	// Find or create a workspace
	err = p.stateStore.Update(func(st *statestore.State) error {
		now := time.Now()

		// Find an available workspace
		for key, ws := range st.Workspaces {
			if ws.Repo == repoName && ws.Status == statestore.WorkspaceStatusAvailable {
				wsPath = ws.Path
				wsName = ws.Name
				needsProvision = !ws.Provisioned

				// Acquire it
				ws.Status = statestore.WorkspaceStatusAcquired
				ws.Purpose = opts.Purpose
				ws.Rev = opts.Rev
				ws.AcquiredByPID = os.Getpid()
				ws.AcquiredAt = now
				ws.CreatedAt = now
				ws.UpdatedAt = now
				st.Workspaces[key] = ws
				return nil
			}
		}

		// No available workspace - create a new one
		wsName = p.nextWorkspaceName(st, repoName)
		wsPath = filepath.Join(p.workspacesDir, repoName, wsName)
		needsCreate = true
		needsProvision = true

		wsKey := repoName + "/" + wsName
		st.Workspaces[wsKey] = statestore.WorkspaceInfo{
			Name:          wsName,
			Repo:          repoName,
			Path:          wsPath,
			Purpose:       opts.Purpose,
			Rev:           opts.Rev,
			Status:        statestore.WorkspaceStatusAcquired,
			AcquiredByPID: os.Getpid(),
			AcquiredAt:    now,
			CreatedAt:     now,
			UpdatedAt:     now,
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
			p.stateStore.Update(func(st *statestore.State) error {
				delete(st.Workspaces, repoName+"/"+wsName)
				return nil
			})
			return "", fmt.Errorf("jj workspace add: %w", err)
		}
	}

	// Edit to the specified revision unless we're already at @.
	actualRev := opts.Rev
	if opts.Rev != "@" {
		if err := p.jj.Edit(wsPath, opts.Rev); err != nil {
			if !strings.Contains(err.Error(), "immutable") {
				return "", fmt.Errorf("jj edit: %w", err)
			}
			var (
				newRev string
				newErr error
			)
			if strings.TrimSpace(opts.NewChangeMessage) != "" {
				newRev, newErr = p.jj.NewChangeWithMessage(wsPath, opts.Rev, opts.NewChangeMessage)
			} else {
				newRev, newErr = p.jj.NewChange(wsPath, opts.Rev)
			}
			if newErr != nil {
				return "", fmt.Errorf("jj new: %w", newErr)
			}
			actualRev = newRev
		}
	}

	if actualRev != opts.Rev {
		if err := p.stateStore.Update(func(st *statestore.State) error {
			wsKey := repoName + "/" + wsName
			if ws, ok := st.Workspaces[wsKey]; ok {
				ws.Rev = actualRev
				ws.UpdatedAt = time.Now()
				st.Workspaces[wsKey] = ws
			}
			return nil
		}); err != nil {
			p.Release(wsPath)
			return "", fmt.Errorf("update workspace rev: %w", err)
		}
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
		p.stateStore.Update(func(st *statestore.State) error {
			wsKey := repoName + "/" + wsName
			if ws, ok := st.Workspaces[wsKey]; ok {
				ws.Provisioned = true
				st.Workspaces[wsKey] = ws
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

	return p.stateStore.Update(func(st *statestore.State) error {
		now := time.Now()
		for key, ws := range st.Workspaces {
			if ws.Path == wsPath {
				ws.Status = statestore.WorkspaceStatusAvailable
				ws.Purpose = ""
				ws.Rev = ""
				ws.AcquiredByPID = 0
				ws.AcquiredAt = time.Time{}
				ws.UpdatedAt = now
				st.Workspaces[key] = ws
				return nil
			}
		}
		return fmt.Errorf("workspace not found: %s", wsPath)
	})
}

// ReleaseByName returns a workspace to the pool by name.
func (p *Pool) ReleaseByName(repoPath, wsName string) error {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
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

// Info contains information about a workspace.
type Info struct {
	// Name is the workspace identifier (e.g., "ws-001").
	Name string

	// Path is the absolute path to the workspace directory.
	Path string

	// Purpose describes why the workspace was acquired.
	Purpose string

	// Rev is the jj revision the workspace was opened to.
	Rev string

	// Status indicates whether the workspace is available or acquired.
	Status Status

	// AcquiredByPID is the process ID that acquired this workspace.
	// Zero if not acquired.
	AcquiredByPID int

	// AcquiredAt is when the workspace was acquired.
	// Zero if not acquired.
	AcquiredAt time.Time

	// CreatedAt is when the workspace acquisition started.
	CreatedAt time.Time

	// UpdatedAt is when the workspace was last released.
	UpdatedAt time.Time
}

// List returns information about all workspaces for the given repository.
//
// The returned slice includes both available and acquired workspaces.

func (p *Pool) List(repoPath string) ([]Info, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	var items []Info

	for _, ws := range st.Workspaces {
		if ws.Repo != repoName {
			continue
		}

		item := Info{
			Name:          ws.Name,
			Path:          ws.Path,
			Purpose:       ws.Purpose,
			Rev:           ws.Rev,
			Status:        ws.Status,
			AcquiredByPID: ws.AcquiredByPID,
			AcquiredAt:    ws.AcquiredAt,
			CreatedAt:     ws.CreatedAt,
			UpdatedAt:     ws.UpdatedAt,
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Status != items[j].Status {
			return workspaceStatusRank(items[i].Status) < workspaceStatusRank(items[j].Status)
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Path < items[j].Path
	})

	return items, nil
}

func workspaceStatusRank(status Status) int {
	switch status {
	case StatusAcquired:
		return 0
	case StatusAvailable:
		return 1
	default:
		return 2
	}
}

// RepoRoot returns the jj repository root for the given path.
//
// This can be used to find the repository root before calling Acquire.
// Returns an error if the path is not inside a jj repository.
func RepoRoot(path string) (string, error) {
	client := jj.New()
	return client.WorkspaceRoot(path)
}

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

	st, err := p.stateStore.Load()
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

	repoPath, found, err := pool.stateStore.RepoPathForWorkspace(root)
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
func (p *Pool) nextWorkspaceName(st *statestore.State, repoName string) string {
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
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return fmt.Errorf("get repo name: %w", err)
	}

	var workspaces []statestore.WorkspaceInfo
	var repoSourcePath string

	// Collect workspaces to destroy and get the source repo path
	err = p.stateStore.Update(func(st *statestore.State) error {
		// Get the source repo path
		if repo, ok := st.Repos[repoName]; ok {
			repoSourcePath = repo.SourcePath
		}

		for key, ws := range st.Workspaces {
			if ws.Repo == repoName {
				workspaces = append(workspaces, ws)
				delete(st.Workspaces, key)
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
