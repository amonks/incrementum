package job

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestRunImplementingStageUpdatesStaleWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/repo"

	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-123", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	workspacePath := t.TempDir()
	updateCalled := false
	commitIDs := []string{"before", "after"}
	commitIndex := 0

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(path string) error {
			if path != workspacePath {
				return fmt.Errorf("unexpected workspace path %q", path)
			}
			updateCalled = true
			return errors.New("not stale")
		},
		CurrentCommitID: func(string) (string, error) {
			if commitIndex >= len(commitIDs) {
				return "", fmt.Errorf("commit id lookup exhausted")
			}
			id := commitIDs[commitIndex]
			commitIndex++
			return id, nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			if !updateCalled {
				return OpencodeRunResult{}, fmt.Errorf("expected update-stale before opencode")
			}
			messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
			if err := os.WriteFile(messagePath, []byte("feat: stale"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-123", ExitCode: 0}, nil
		},
	}

	item := todo.Todo{
		ID:          "todo-123",
		Title:       "Test todo",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, nil, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}
	if !updateCalled {
		t.Fatalf("expected update-stale to be called")
	}
	if result.Job.Stage != StageTesting {
		t.Fatalf("expected stage %q, got %q", StageTesting, result.Job.Stage)
	}
}

func TestRunReviewingStageUpdatesStaleWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/repo-review"

	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 11, 10, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-456", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	workspacePath := t.TempDir()
	updateCalled := false
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(path string) error {
			if path != workspacePath {
				return fmt.Errorf("unexpected workspace path %q", path)
			}
			updateCalled = true
			return errors.New("not stale")
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			if !updateCalled {
				return OpencodeRunResult{}, fmt.Errorf("expected update-stale before opencode")
			}
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-456", ExitCode: 0}, nil
		},
	}

	item := todo.Todo{
		ID:          "todo-456",
		Title:       "Review todo",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "fix: stale review"
	updated, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, nil, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}
	if !updateCalled {
		t.Fatalf("expected update-stale to be called")
	}
	if updated.Stage != StageCommitting {
		t.Fatalf("expected stage %q, got %q", StageCommitting, updated.Stage)
	}
}
