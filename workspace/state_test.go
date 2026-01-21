package workspace

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestStateStore_LoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	st, err := store.load()
	if err != nil {
		t.Fatalf("failed to load empty state: %v", err)
	}

	if st == nil {
		t.Fatal("expected non-nil state")
	}

	if len(st.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(st.Repos))
	}

	if len(st.Workspaces) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(st.Workspaces))
	}
}

func TestStateStore_LoadEmpty_Opencode(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	st, err := store.load()
	if err != nil {
		t.Fatalf("failed to load empty state: %v", err)
	}

	if st.OpencodeDaemons == nil {
		t.Fatal("expected opencode daemons map")
	}

	if st.OpencodeSessions == nil {
		t.Fatal("expected opencode sessions map")
	}
}

func TestStateStore_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	st := &state{
		Repos: map[string]repoInfo{
			"my-project": {SourcePath: "/Users/test/my-project"},
		},
		Workspaces: map[string]workspaceInfo{
			"my-project/ws-001": {
				Name:        "ws-001",
				Repo:        "my-project",
				Path:        "/Users/test/.local/share/incrementum/workspaces/my-project/ws-001",
				Purpose:     "initial sync",
				Status:      StatusAcquired,
				Provisioned: true,
			},
		},
	}

	if err := store.save(st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	loaded, err := store.load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(loaded.Repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(loaded.Repos))
	}

	if loaded.Repos["my-project"].SourcePath != "/Users/test/my-project" {
		t.Error("repo source path mismatch")
	}

	if len(loaded.Workspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(loaded.Workspaces))
	}

	ws := loaded.Workspaces["my-project/ws-001"]
	if ws.Name != "ws-001" {
		t.Errorf("expected name ws-001, got %s", ws.Name)
	}
	if ws.Purpose != "initial sync" {
		t.Errorf("expected purpose to persist, got %q", ws.Purpose)
	}
	if ws.Status != StatusAcquired {
		t.Errorf("expected status claimed, got %s", ws.Status)
	}
}

func TestStateStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	// Use update to atomically modify state
	err := store.update(func(st *state) error {
		st.Repos["my-project"] = repoInfo{SourcePath: "/test/path"}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to update state: %v", err)
	}

	// Verify the update
	loaded, err := store.load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loaded.Repos["my-project"].SourcePath != "/test/path" {
		t.Error("update did not persist")
	}
}

func TestStateStore_ConcurrentUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	// Initialize with a counter
	err := store.update(func(st *state) error {
		st.Repos["counter"] = repoInfo{SourcePath: "0"}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to init state: %v", err)
	}

	// Run concurrent updates
	var wg sync.WaitGroup
	numGoroutines := 10
	incrementsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				err := store.update(func(st *state) error {
					// Just verify we can read/write without corruption
					_ = st.Repos["counter"]
					st.Repos["counter"] = repoInfo{SourcePath: "updated"}
					return nil
				})
				if err != nil {
					t.Errorf("concurrent update failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	// Verify final state is valid
	loaded, err := store.load()
	if err != nil {
		t.Fatalf("failed to load final state: %v", err)
	}

	if loaded.Repos["counter"].SourcePath != "updated" {
		t.Error("final state is corrupted")
	}
}

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/Users/test/my-project", "users-test-my-project"},
		{"/Users/test/My Project", "users-test-my-project"},
		{"/home/user/some/deep/path", "home-user-some-deep-path"},
	}

	for _, tt := range tests {
		result := sanitizeRepoName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeRepoName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestStateStore_GetOrCreateRepoName(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	// First call should create the repo
	name1, err := store.getOrCreateRepoName("/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	// Second call with same path should return same name
	name2, err := store.getOrCreateRepoName("/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	if name1 != name2 {
		t.Errorf("expected same name, got %q and %q", name1, name2)
	}

	// Different path that sanitizes to same name should get a suffix
	name3, err := store.getOrCreateRepoName("/Users/test/my/project") // different path, could collide
	if err != nil {
		t.Fatalf("failed to get repo name: %v", err)
	}

	// Should either be different or if it collides, have a suffix
	if name3 == name1 {
		t.Error("collision not handled - different paths got same name")
	}
}

func TestStateStore_RepoPathForWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	repoName, err := store.getOrCreateRepoName("/Users/test/my-project")
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	wsPath := filepath.Join("/tmp/workspaces", repoName, "ws-001")
	if err := store.update(func(st *state) error {
		st.Workspaces[repoName+"/ws-001"] = workspaceInfo{
			Name: "ws-001",
			Repo: repoName,
			Path: wsPath,
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to add workspace: %v", err)
	}

	resolved, found, err := store.repoPathForWorkspace(wsPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected workspace to be found")
	}
	if resolved != "/Users/test/my-project" {
		t.Fatalf("expected repo path, got %q", resolved)
	}

	_, found, err = store.repoPathForWorkspace(filepath.Join("/tmp/workspaces", repoName, "ws-999"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected workspace to be missing")
	}
}

func TestStateStore_RepoPathForWorkspace_MissingRepo(t *testing.T) {
	tmpDir := t.TempDir()
	store := newStateStore(tmpDir)

	wsPath := filepath.Join("/tmp/workspaces", "missing", "ws-001")
	if err := store.update(func(st *state) error {
		st.Workspaces["missing/ws-001"] = workspaceInfo{
			Name: "ws-001",
			Repo: "missing",
			Path: wsPath,
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to add workspace: %v", err)
	}

	_, found, err := store.repoPathForWorkspace(wsPath)
	if !found {
		t.Fatal("expected workspace to be found")
	}
	if err == nil {
		t.Fatal("expected error for missing repo path")
	}
}
