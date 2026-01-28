package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveRunStdinUsesPromptWhenStdinNil(t *testing.T) {
	opts := RunOptions{Prompt: "Hello"}

	got := resolveRunStdin(opts)

	reader, ok := got.(*strings.Reader)
	if !ok {
		t.Fatalf("expected strings.Reader, got %T", got)
	}
	if reader.Len() != len(opts.Prompt) {
		t.Fatalf("expected reader length %d, got %d", len(opts.Prompt), reader.Len())
	}
}

func TestResolveRunStdinUsesOSStdinWhenPromptEmpty(t *testing.T) {
	opts := RunOptions{}

	got := resolveRunStdin(opts)

	if got != os.Stdin {
		t.Fatalf("expected os.Stdin, got %T", got)
	}
}

func TestResolveRunStdinPrefersProvidedStdin(t *testing.T) {
	stdin := strings.NewReader("input")
	opts := RunOptions{Prompt: "Hello", Stdin: stdin}

	got := resolveRunStdin(opts)

	if got != stdin {
		t.Fatalf("expected provided stdin, got %T", got)
	}
}

func TestEnsureSessionUsesWorkDirForStorageLookup(t *testing.T) {
	store := openTestStore(t)
	baseDir := t.TempDir()
	repoPath := filepath.Join(baseDir, "repo")
	workDir := filepath.Join(baseDir, "workspace")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create work dir: %v", err)
	}

	projectID := "proj_workspace"
	sessionID := "ses_workspace"
	startedAt := time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)
	createdAt := startedAt.Add(2 * time.Second)

	projectDir := filepath.Join(store.storage.Root, "storage", "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}
	writeStorageJSON(t, filepath.Join(projectDir, projectID+".json"), map[string]any{
		"id":       projectID,
		"worktree": workDir,
	})

	sessionDir := filepath.Join(store.storage.Root, "storage", "session", projectID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("create session dir: %v", err)
	}
	writeStorageJSON(t, filepath.Join(sessionDir, sessionID+".json"), map[string]any{
		"id":        sessionID,
		"projectID": projectID,
		"directory": workDir,
		"time": map[string]any{
			"created": createdAt.UnixMilli(),
		},
	})

	if _, err := store.ensureSession(repoPath, workDir, startedAt, "prompt"); err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	found, err := store.FindSession(repoPath, sessionID)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if found.ID != sessionID {
		t.Fatalf("expected session ID %q, got %q", sessionID, found.ID)
	}
}

func writeStorageJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
