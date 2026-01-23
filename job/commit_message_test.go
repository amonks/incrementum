package job

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCommitMessageDeletesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commit-message")
	if err := os.WriteFile(path, []byte("feat: add widgets\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	message, err := readCommitMessage(path)
	if err != nil {
		t.Fatalf("read commit message: %v", err)
	}
	if message != "feat: add widgets" {
		t.Fatalf("expected message %q, got %q", "feat: add widgets", message)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected commit message file to be deleted")
	}
}

func TestReadCommitMessageDeletesEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commit-message")
	if err := os.WriteFile(path, []byte("\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	if _, err := readCommitMessage(path); err == nil {
		t.Fatalf("expected error for empty commit message")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected commit message file to be deleted")
	}
}

func TestReadCommitMessageFallsBackToRepoRoot(t *testing.T) {
	workspaceDir := t.TempDir()
	repoDir := t.TempDir()
	primary := filepath.Join(workspaceDir, commitMessageFilename)
	fallback := filepath.Join(repoDir, commitMessageFilename)

	if err := os.WriteFile(fallback, []byte("fix: keep workspace\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	message, err := readCommitMessageWithFallback(primary, fallback)
	if err != nil {
		t.Fatalf("read commit message: %v", err)
	}
	if message != "fix: keep workspace" {
		t.Fatalf("expected message %q, got %q", "fix: keep workspace", message)
	}

	if _, err := os.Stat(fallback); !os.IsNotExist(err) {
		t.Fatalf("expected commit message fallback to be deleted")
	}
}
