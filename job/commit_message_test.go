package job

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amonks/incrementum/todo"
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

func TestReadCommitMessageMissingExplainsPath(t *testing.T) {
	workspaceDir := t.TempDir()
	primary := filepath.Join(workspaceDir, commitMessageFilename)

	_, err := readCommitMessage(primary)
	if err == nil {
		t.Fatal("expected missing commit message error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if !strings.Contains(err.Error(), "commit message missing") {
		t.Fatalf("expected context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), primary) {
		t.Fatalf("expected paths in error, got %v", err)
	}
}

func TestFormatCommitMessage(t *testing.T) {
	item := todo.Todo{
		ID:          "todo-555",
		Title:       "Shore up logs",
		Description: "Improve log formatting for the job runner.",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityLow,
	}
	message := "feat: reflow logs\n\nEnsure log output is wrapped and indented for clarity."

	formatted := formatCommitMessage(item, message)
	if !strings.HasPrefix(formatted, "feat: reflow logs") {
		t.Fatalf("expected commit summary at start, got %q", formatted)
	}

	checks := []string{
		"Here is a generated commit message:",
		"    Ensure log output is wrapped and indented for clarity.",
		"This commit is a step towards implementing this todo:",
		"    ID: todo-555",
		"    Title: Shore up logs",
		"    Type: task",
		"    Priority: 3 (low)",
		"    Description:",
		"        Improve log formatting for the job runner.",
	}
	for _, check := range checks {
		if !strings.Contains(formatted, check) {
			t.Fatalf("expected commit message to include %q, got %q", check, formatted)
		}
	}
}
