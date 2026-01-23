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
