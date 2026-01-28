package job

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestRunImplementingStage_MissingCommitMessageExplainsContext(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	current, err := manager.Create("todo-1", now, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-1",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls == 1 {
				return "before", nil
			}
			return "after", nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			return OpencodeRunResult{SessionID: "ses-123", ExitCode: 0}, nil
		},
	}

	_, err = runImplementingStage(manager, current, item, repoPath, repoPath, opts, nil, "")
	if err == nil {
		t.Fatal("expected missing commit message error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if !strings.Contains(err.Error(), "commit message missing after opencode implementation") {
		t.Fatalf("expected context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "opencode session ses-123") {
		t.Fatalf("expected session context, got %v", err)
	}
	if !strings.Contains(err.Error(), "before") || !strings.Contains(err.Error(), "after") {
		t.Fatalf("expected commit change context, got %v", err)
	}
	if !strings.Contains(err.Error(), "was instructed to write") {
		t.Fatalf("expected opencode instruction context, got %v", err)
	}
	if !strings.Contains(err.Error(), commitMessageFilename) {
		t.Fatalf("expected error to mention commit message file, got %v", err)
	}
}

func TestRunImplementingStageFailedOpencodeRestoresAndReportsContext(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 6, 0, time.UTC)
	current, err := manager.Create("todo-restore", now, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-restore",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	commitCalls := 0
	restoreCalled := false
	restoreCommit := ""
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls == 1 {
				return "before", nil
			}
			return "after", nil
		},
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			return OpencodeRunResult{SessionID: "ses-789", ExitCode: -1}, nil
		},
		RestoreWorkspace: func(_ string, commitID string) error {
			restoreCalled = true
			restoreCommit = commitID
			return nil
		},
		OpencodeAgent: "gpt-5.2-codex",
	}

	_, err = runImplementingStage(manager, current, item, repoPath, repoPath, opts, nil, "")
	if err == nil {
		t.Fatal("expected opencode failure error")
	}
	if !restoreCalled {
		t.Fatalf("expected restore to be called")
	}
	if restoreCommit != "before" {
		t.Fatalf("expected restore commit to be before, got %q", restoreCommit)
	}
	message := err.Error()
	if !strings.Contains(message, "opencode implement failed with exit code -1") {
		t.Fatalf("expected exit code context, got %v", message)
	}
	if !strings.Contains(message, "session ses-789") {
		t.Fatalf("expected session context, got %v", message)
	}
	if !strings.Contains(message, "agent \"gpt-5.2-codex\"") {
		t.Fatalf("expected agent context, got %v", message)
	}
	if !strings.Contains(message, "prompt prompt-implementation.tmpl") {
		t.Fatalf("expected prompt context, got %v", message)
	}
	if !strings.Contains(message, "before before") || !strings.Contains(message, "after after") {
		t.Fatalf("expected commit context, got %v", message)
	}
	if !strings.Contains(message, "restored before") {
		t.Fatalf("expected restore context, got %v", message)
	}
}

func TestRunImplementingStageTreatsEmptyDiffAsNoChange(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	current, err := manager.Create("todo-2", now, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-2",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	messagePath := filepath.Join(repoPath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte("feat: example\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls == 1 {
				return "before", nil
			}
			return "after", nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "\n", nil
		},
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			return OpencodeRunResult{SessionID: "ses-456", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, nil, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Changed {
		t.Fatalf("expected no change, got changed")
	}
	if _, err := os.Stat(messagePath); !os.IsNotExist(err) {
		t.Fatalf("expected commit message to be deleted")
	}
}

func TestRunImplementingStageTreatsZeroDiffStatAsNoChange(t *testing.T) {
	repoPath := t.TempDir()
	stateDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	now := time.Date(2026, time.January, 2, 3, 4, 6, 0, time.UTC)
	current, err := manager.Create("todo-3", now, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-3",
		Title:    "Example",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	messagePath := filepath.Join(repoPath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte("feat: example\n"), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time { return now },
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls == 1 {
				return "before", nil
			}
			return "after", nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "0 files changed, 0 insertions(+), 0 deletions(-)\n", nil
		},
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			return OpencodeRunResult{SessionID: "ses-789", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, current, item, repoPath, repoPath, opts, nil, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Changed {
		t.Fatalf("expected no change, got changed")
	}
	if _, err := os.Stat(messagePath); !os.IsNotExist(err) {
		t.Fatalf("expected commit message to be deleted")
	}
}
