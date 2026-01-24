package job

import (
	"errors"
	"os"
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
	current, err := manager.Create("todo-1", now)
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
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			return OpencodeRunResult{SessionID: "ses-123", ExitCode: 0}, nil
		},
	}

	_, err = runImplementingStage(manager, current, item, repoPath, repoPath, opts)
	if err == nil {
		t.Fatal("expected missing commit message error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if !strings.Contains(err.Error(), "commit message missing after opencode implementation") {
		t.Fatalf("expected context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), commitMessageFilename) {
		t.Fatalf("expected error to mention commit message file, got %v", err)
	}
}
