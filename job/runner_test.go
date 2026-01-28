package job

import (
	"encoding/json"
	"errors"
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
	created, err := manager.Create("todo-789", startedAt, "")
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
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
			if err := os.WriteFile(messagePath, []byte("feat: step"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-789", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, nil, "")
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

func TestRunImplementingStageNoChangesSkipsTesting(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := "/Users/test/repo"
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 5, 0, 0, time.UTC)
	created, err := manager.Create("todo-790", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-790",
		Title:       "No changes",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	messagePath := filepath.Join(workspacePath, commitMessageFilename)
	if err := os.WriteFile(messagePath, []byte("old message"), 0o644); err != nil {
		t.Fatalf("seed commit message: %v", err)
	}

	commitIDs := []string{"same", "same"}
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
		DiffStat: func(string, string, string) (string, error) {
			return "", fmt.Errorf("diff stat should not be called")
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			return OpencodeRunResult{SessionID: "oc-790", ExitCode: 0}, nil
		},
	}

	result, err := runImplementingStage(manager, created, item, repoPath, workspacePath, opts, nil, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}
	if result.Changed {
		t.Fatalf("expected no change detected")
	}
	if result.CommitMessage != "" {
		t.Fatalf("expected empty commit message, got %q", result.CommitMessage)
	}
	if result.Job.Stage != StageReviewing {
		t.Fatalf("expected stage %q, got %q", StageReviewing, result.Job.Stage)
	}
	if _, err := os.Stat(messagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected commit message removed, got %v", err)
	}
}

func TestRunImplementingStageIncludesCommitMessageInstructionWithFeedback(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 10, 0, 0, time.UTC)
	created, err := manager.Create("todo-111", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	created.Feedback = "Tests failed"

	item := todo.Todo{
		ID:          "todo-111",
		Title:       "Retry with feedback",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	previousMessage := "feat: earlier draft"

	commitCalls := 0
	var seenPrompt string
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		CurrentCommitID: func(string) (string, error) {
			commitCalls++
			if commitCalls > 2 {
				return "", fmt.Errorf("commit id lookup exhausted")
			}
			return "same", nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			seenPrompt = runOpts.Prompt
			return OpencodeRunResult{SessionID: "oc-111", ExitCode: 0}, nil
		},
	}

	_, err = runImplementingStage(manager, created, item, repoPath, workspacePath, opts, nil, previousMessage)
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, ".incrementum-commit-message") {
		t.Fatalf("expected prompt to request commit message, got %q", seenPrompt)
	}
	if !strings.Contains(seenPrompt, previousMessage) {
		t.Fatalf("expected prompt to include previous commit message, got %q", seenPrompt)
	}
}

func TestRunImplementingStageIncludesCommitLog(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 11, 20, 0, 0, time.UTC)
	created, err := manager.Create("todo-212", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-212",
		Title:       "Show commit log",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityLow,
	}

	commitIDs := []string{"same", "same"}
	commitIndex := 0
	commitLog := []CommitLogEntry{{ID: "commit-42", Message: "feat: initial work"}}

	var seenPrompt string
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
			seenPrompt = runOpts.Prompt
			return OpencodeRunResult{SessionID: "oc-212", ExitCode: 0}, nil
		},
	}

	_, err = runImplementingStage(manager, created, item, repoPath, workspacePath, opts, commitLog, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, "commit-42") {
		t.Fatalf("expected prompt to include commit id, got %q", seenPrompt)
	}
	if !strings.Contains(seenPrompt, "feat: initial work") {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
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
	created, err := manager.Create("todo-456", startedAt, "")
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

	commitLog := []CommitLogEntry{{ID: "commit-abc", Message: "feat: previous"}}

	updated, err := runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, commitLog, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
	}
	if !strings.Contains(seenPrompt, "commit-abc") {
		t.Fatalf("expected prompt to include commit log, got %q", seenPrompt)
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
	created, err := manager.Create("todo-987", startedAt, "")
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

	_, err = runReviewingStage(manager, created, item, repoPath, workspacePath, opts, "", nil, reviewScopeStep)
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

func TestRunReviewingStageMissingCommitMessageExplainsContext(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 40, 0, 0, time.UTC)
	current, err := manager.Create("todo-123", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-123",
		Title:       "Review commit missing message",
		Description: "",
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	calledOpencode := false
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			calledOpencode = true
			return OpencodeRunResult{SessionID: "oc-123", ExitCode: 0}, nil
		},
	}

	_, err = runReviewingStage(manager, current, item, repoPath, workspacePath, opts, "", nil, reviewScopeStep)
	if err == nil {
		t.Fatal("expected missing commit message error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got %v", err)
	}
	if calledOpencode {
		t.Fatalf("expected review to stop before opencode")
	}
	if !strings.Contains(err.Error(), "commit message missing before opencode review") {
		t.Fatalf("expected context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "opencode implementation") {
		t.Fatalf("expected author context, got %v", err)
	}
	if !strings.Contains(err.Error(), commitMessageFilename) {
		t.Fatalf("expected commit message path context, got %v", err)
	}
}

func TestRunReviewingStageInjectsCommitMessageWhenTemplateMissing(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	promptDir := filepath.Join(workspacePath, ".incrementum", "templates")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}
	customPrompt := "Review the changes in the jujutsu working tree."
	if err := os.WriteFile(filepath.Join(promptDir, "prompt-commit-review.tmpl"), []byte(customPrompt), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 45, 0, 0, time.UTC)
	created, err := manager.Create("todo-654", startedAt, "")
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

	_, err = runReviewingStage(manager, created, item, repoPath, workspacePath, opts, commitMessage, nil, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if !strings.Contains(seenPrompt, "Commit message\n\n    "+commitMessage) {
		t.Fatalf("expected prompt to include injected commit message block, got %q", seenPrompt)
	}
	if !strings.Contains(seenPrompt, commitMessage) {
		t.Fatalf("expected prompt to include commit message, got %q", seenPrompt)
	}
}

func TestRunCommittingStageFormatsCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 0, 0, 0, time.UTC)
	current, err := manager.Create("todo-333", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	append := OpencodeSession{Purpose: "implement", ID: "ses-333"}
	current, err = manager.Update(current.ID, UpdateOptions{AppendOpencodeSession: &append}, startedAt)
	if err != nil {
		t.Fatalf("append opencode session: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-333",
		Title:       "Expand commit message",
		Description: "Add todo metadata and transcripts.",
		Status:      todo.StatusOpen,
		Type:        todo.TypeTask,
		Priority:    todo.PriorityHigh,
	}

	var captured string
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		OpencodeTranscripts: func(repoPath string, sessions []OpencodeSession) ([]OpencodeTranscript, error) {
			return []OpencodeTranscript{{Purpose: "implement", ID: "ses-333", Transcript: "Planning\n"}}, nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-333", nil
		},
	}
	opts.Commit = func(string, message string) error {
		captured = message
		return nil
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        &RunResult{},
		CommitMessage: "feat: expand commit metadata",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	checks := []string{
		"feat: expand commit metadata",
		"Here is a generated commit message:",
		"This commit is a step towards implementing this todo:",
		"    ID: todo-333",
		"    Title: Expand commit message",
		"    Type: task",
		"    Priority: 1 (high)",
		"    Description:",
		"        Add todo metadata and transcripts.",
	}
	for _, check := range checks {
		if !strings.Contains(captured, check) {
			t.Fatalf("expected commit message to include %q, got %q", check, captured)
		}
	}
}

func TestRunCommittingStageLogsFormattedCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()
	eventsDir := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 5, 0, 0, time.UTC)
	current, err := manager.Create("todo-commit-log", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:          "todo-commit-log",
		Title:       "Log commit message",
		Description: "Ensure final commit message logs use the log width.",
		Status:      todo.StatusOpen,
		Type:        todo.TypeTask,
		Priority:    todo.PriorityMedium,
	}

	log, err := OpenEventLog(current.ID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close event log: %v", err)
		}
	}()

	message := "feat: log commit message"
	expectedLogMessage := formatCommitMessageWithWidth(item, message, lineWidth-subdocumentIndent)

	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-log", nil
		},
		Commit: func(string, string) error {
			return nil
		},
		EventLog: log,
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        &RunResult{},
		CommitMessage: message,
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	path := filepath.Join(eventsDir, current.ID+".jsonl")
	events := readEventLogFile(t, path)
	if len(events) == 0 {
		t.Fatal("expected event log entries")
	}

	var commitEvent commitMessageEventData
	for _, event := range events {
		if event.Name != jobEventCommitMessage {
			continue
		}
		if err := json.Unmarshal([]byte(event.Data), &commitEvent); err != nil {
			t.Fatalf("decode commit message event: %v", err)
		}
		break
	}
	if commitEvent.Message == "" {
		t.Fatalf("expected commit message event, got %v", events)
	}
	if commitEvent.Message != expectedLogMessage {
		t.Fatalf("expected log message %q, got %q", expectedLogMessage, commitEvent.Message)
	}
	if !commitEvent.Preformatted {
		t.Fatalf("expected preformatted commit message, got %#v", commitEvent)
	}
}

func TestRunCommittingStageSkipsEmptyChange(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 7, 0, 0, time.UTC)
	current, err := manager.Create("todo-empty-change", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-empty-change",
		Title:    "Skip empty change",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	commitCalls := 0
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "0 files changed, 0 insertions(+), 0 deletions(-)\n", nil
		},
		Commit: func(string, string) error {
			commitCalls++
			return nil
		},
	}

	updated, err := runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        &RunResult{},
		CommitMessage: "feat: nothing to commit",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}
	if commitCalls != 0 {
		t.Fatalf("expected no commit attempt, got %d", commitCalls)
	}
	if updated.Stage != StageImplementing {
		t.Fatalf("expected stage %q, got %q", StageImplementing, updated.Stage)
	}
}

func TestRunCommittingStageOmitsCommitLog(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 10, 0, 0, time.UTC)
	current, err := manager.Create("todo-commit-log-template", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-commit-log-template",
		Title:    "Include commit log",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	result := &RunResult{CommitLog: []CommitLogEntry{{ID: "commit-prev", Message: "feat: previous step"}}}
	var captured string
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		OpencodeTranscripts: func(string, []OpencodeSession) ([]OpencodeTranscript, error) {
			return nil, nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-new", nil
		},
		Commit: func(string, string) error {
			return nil
		},
	}
	opts.Commit = func(string, message string) error {
		captured = message
		return nil
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        result,
		CommitMessage: "feat: include commit log",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	if strings.Contains(captured, "commit-prev") {
		t.Fatalf("expected commit message to omit commit log, got %q", captured)
	}
}

func TestRunCommittingStageOmitsEmptyCommitLog(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 12, 0, 0, time.UTC)
	current, err := manager.Create("todo-empty-commit-log", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-empty-commit-log",
		Title:    "No commit log",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	result := &RunResult{}
	var captured string
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		OpencodeTranscripts: func(string, []OpencodeSession) ([]OpencodeTranscript, error) {
			return nil, nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-100", nil
		},
		Commit: func(string, string) error {
			return nil
		},
	}
	opts.Commit = func(string, message string) error {
		captured = message
		return nil
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        result,
		CommitMessage: "feat: first commit",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	if strings.Contains(captured, "Commit log:") {
		t.Fatalf("expected commit message to omit commit log, got %q", captured)
	}
}

func TestDiffStatHasChangesDetectsEmptySummaries(t *testing.T) {
	cases := []struct {
		name     string
		diffStat string
		changed  bool
	}{
		{
			name:     "empty output",
			diffStat: "\n\n",
			changed:  false,
		},
		{
			name:     "no changes line",
			diffStat: "No changes.\n",
			changed:  false,
		},
		{
			name:     "zero summary after header",
			diffStat: "Working copy is clean\n0 files changed, 0 insertions(+), 0 deletions(-)\n",
			changed:  false,
		},
		{
			name:     "header without file stats",
			diffStat: "Working copy changes:\n\n",
			changed:  false,
		},
		{
			name:     "summary with changes",
			diffStat: "2 files changed, 3 insertions(+), 1 deletion(-)\n",
			changed:  true,
		},
		{
			name:     "file changes",
			diffStat: "file.txt | 2 +-\n1 file changed, 1 insertion(+), 1 deletion(-)\n",
			changed:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := diffStatHasChanges(tc.diffStat); got != tc.changed {
				t.Fatalf("expected changed=%t, got %t", tc.changed, got)
			}
		})
	}
}

func TestRunCommittingStageAppendsCommitLog(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 15, 0, 0, time.UTC)
	current, err := manager.Create("todo-commit-log", startedAt, "")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-commit-log",
		Title:    "Commit log",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	result := &RunResult{}
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		DiffStat: func(string, string, string) (string, error) {
			return "file.txt | 1 +\n", nil
		},
		OpencodeTranscripts: func(string, []OpencodeSession) ([]OpencodeTranscript, error) {
			return nil, nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-456", nil
		},
		Commit: func(string, string) error {
			return nil
		},
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        result,
		CommitMessage: "feat: log commit",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	if len(result.CommitLog) != 1 {
		t.Fatalf("expected 1 commit log entry, got %d", len(result.CommitLog))
	}
	if result.CommitLog[0].ID != "commit-456" {
		t.Fatalf("expected commit id %q, got %q", "commit-456", result.CommitLog[0].ID)
	}
	if !strings.Contains(result.CommitLog[0].Message, "feat: log commit") {
		t.Fatalf("expected commit log to include message, got %q", result.CommitLog[0].Message)
	}
}
