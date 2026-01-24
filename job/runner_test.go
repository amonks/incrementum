package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestTestingStageOutcomeFailure(t *testing.T) {
	results := []TestCommandResult{
		{Command: "go test ./...", ExitCode: 1},
		{Command: "golangci-lint run", ExitCode: 0},
	}

	stage, feedback := testingStageOutcome(results)

	if stage != StageImplementing {
		t.Fatalf("expected stage %q, got %q", StageImplementing, stage)
	}

	expected := FormatTestFeedback([]TestCommandResult{{Command: "go test ./...", ExitCode: 1}})
	if feedback != expected {
		t.Fatalf("expected feedback %q, got %q", expected, feedback)
	}
}

func TestTestingStageOutcomeSuccess(t *testing.T) {
	results := []TestCommandResult{{Command: "go test ./...", ExitCode: 0}}

	stage, feedback := testingStageOutcome(results)

	if stage != StageReviewing {
		t.Fatalf("expected stage %q, got %q", StageReviewing, stage)
	}
	if feedback != "" {
		t.Fatalf("expected empty feedback, got %q", feedback)
	}
}

func TestRunImplementingStageReadsCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-789", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-789",
		Title:       "Commit message",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitIDs := []string{"before", "after"}
	commitIndex := 0

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		CurrentCommitID: func(string) (string, error) {
			if commitIndex >= len(commitIDs) {
				return "", fmt.Errorf("commit id lookup exhausted")
			}
			id := commitIDs[commitIndex]
			commitIndex++
			return id, nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
			if err := os.WriteFile(messagePath, []byte("feat: step"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-789", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts)
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}
	if !result.Changed {
		t.Fatalf("expected change detected")
	}
	if result.CommitMessage != "feat: step" {
		t.Fatalf("expected commit message %q, got %q", "feat: step", result.CommitMessage)
	}
	if result.Job.Stage != StageTesting {
		t.Fatalf("expected stage %q, got %q", StageTesting, result.Job.Stage)
	}
}

func TestRunReviewingStagePassesCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-456", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-456",
		Title:       "Review commit",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "feat: add review message"
	var seenPrompt string
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			seenPrompt = runOpts.Prompt
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-456", ExitCode: 0}, nil
		},
	}

	updated, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
	}
	if updated.Stage != StageCommitting {
		t.Fatalf("expected stage %q, got %q", StageCommitting, updated.Stage)
	}
}

func TestRunReviewingStageReadsCommitMessageFile(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-987", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-987",
		Title:       "Review commit file",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "fix: include review prompt"
	messagePath := filepath.Join(workspacePath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte(commitMessage), 0o644); err != nil {
		t.Fatalf("write commit message: %v", err)
	}

	var seenPrompt string
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			seenPrompt = runOpts.Prompt
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-789", ExitCode: 0}, nil
		},
	}

	_, err = runReviewingStage(manager, created, item, repoPath, workspacePath, opts, "", reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
	}
	if _, err := os.Stat(messagePath); !os.IsNotExist(err) {
		t.Fatalf("expected commit message file to be deleted")
	}
}

func TestRunReviewingStageInjectsCommitMessageWhenTemplateMissing(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	promptDir := filepath.Join(workspacePath, ".incrementum", "prompts")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}
	customPrompt := "Review the changes in the working tree."
	if err := os.WriteFile(filepath.Join(promptDir, "review.tmpl"), []byte(customPrompt), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 45, 0, 0, time.UTC)
	created, err := manager.Create("todo-654", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-654",
		Title:       "Review commit fallback",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	commitMessage := "fix: inject review prompt"
	var seenPrompt string
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			seenPrompt = runOpts.Prompt
			if err := os.WriteFile(feedbackPath, []byte("ACCEPT\n"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-654", ExitCode: 0}, nil
		},
	}

	_, err = runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, "<commit_message>") {
		t.Fatalf("expected prompt to include injected commit message tag, got %q", seenPrompt)
	}
	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
	}
}
