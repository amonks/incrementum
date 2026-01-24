package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/todo"
)

type captureLogger struct {
	prompts []PromptLog
	commits []CommitMessageLog
	reviews []ReviewLog
	tests   []TestLog
}

func (logger *captureLogger) Prompt(entry PromptLog) {
	logger.prompts = append(logger.prompts, entry)
}

func (logger *captureLogger) CommitMessage(entry CommitMessageLog) {
	logger.commits = append(logger.commits, entry)
}

func (logger *captureLogger) Review(entry ReviewLog) {
	logger.reviews = append(logger.reviews, entry)
}

func (logger *captureLogger) Tests(entry TestLog) {
	logger.tests = append(logger.tests, entry)
}

func TestConsoleLoggerFormatsEntries(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf)

	logger.Prompt(PromptLog{
		Purpose:    "implement",
		Template:   "prompt-implementation.tmpl",
		Prompt:     "Implement the change.\nAnd keep going.",
		Transcript: "Plan the work.\nThen execute.",
	})
	logger.CommitMessage(CommitMessageLog{Label: "Draft", Message: "feat: draft commit"})
	logger.Tests(TestLog{Results: []TestCommandResult{{Command: "go test ./...", ExitCode: 1}}})
	logger.Review(ReviewLog{Purpose: "review", Feedback: ReviewFeedback{Outcome: ReviewOutcomeRequestChanges, Details: "Add tests."}})
	logger.CommitMessage(CommitMessageLog{Label: "Final", Message: "feat: final commit"})

	output := stripANSI(buf.String())
	checks := []string{
		"Implementation prompt:",
		"Implement the change",
		"Opencode transcript:",
		"Plan the work.",
		"Draft commit message:",
		"feat: draft commit",
		"go test ./...",
		"Code review result:",
		"Add tests.",
		"Final commit message:",
		"feat: final commit",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to include %q, got %q", check, output)
		}
	}
}

func TestConsoleLoggerReflowsParagraphs(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf)

	logger.Prompt(PromptLog{
		Purpose: "implement",
		Prompt:  "First paragraph line one.\nSecond line stays in the same paragraph.\n\nSecond paragraph follows next.",
	})

	output := stripANSI(buf.String())
	if !strings.Contains(output, "First paragraph line one. Second line stays in the same paragraph.") {
		t.Fatalf("expected paragraph lines to reflow, got %q", output)
	}
	if !strings.Contains(output, "\n        \n") {
		t.Fatalf("expected paragraph break to be preserved, got %q", output)
	}
}

func TestRunImplementingStageLogsPromptAndCommitMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 0, 0, 0, time.UTC)
	current, err := manager.Create("todo-logger", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-logger",
		Title:    "Log prompt",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	logger := &captureLogger{}
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
			if err := os.WriteFile(messagePath, []byte("feat: log message"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-log", ExitCode: 0}, nil
		},
		Logger: logger,
	}

	_, err = runImplementingStage(manager, current, item, repoPath, workspacePath, opts, nil, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}

	if len(logger.prompts) != 1 {
		t.Fatalf("expected 1 prompt log, got %d", len(logger.prompts))
	}
	if logger.prompts[0].Purpose != "implement" {
		t.Fatalf("expected prompt purpose implement, got %q", logger.prompts[0].Purpose)
	}
	if logger.prompts[0].Template != "prompt-implementation.tmpl" {
		t.Fatalf("expected prompt template, got %q", logger.prompts[0].Template)
	}
	if !strings.Contains(logger.prompts[0].Prompt, "write a multi-line commit message") {
		t.Fatalf("expected prompt to include commit message instructions")
	}

	if len(logger.commits) != 1 {
		t.Fatalf("expected 1 commit log, got %d", len(logger.commits))
	}
	if logger.commits[0].Label != "Draft" {
		t.Fatalf("expected draft commit label, got %q", logger.commits[0].Label)
	}
	if logger.commits[0].Message != "feat: log message" {
		t.Fatalf("expected commit message, got %q", logger.commits[0].Message)
	}
}

func TestRunImplementingStageUsesFeedbackPrompt(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 10, 0, 0, time.UTC)
	current, err := manager.Create("todo-feedback", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	current.Feedback = "Add coverage."

	item := todo.Todo{
		ID:       "todo-feedback",
		Title:    "Respond to feedback",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	logger := &captureLogger{}
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
			if err := os.WriteFile(messagePath, []byte("feat: respond"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-feedback", ExitCode: 0}, nil
		},
		Logger: logger,
	}

	_, err = runImplementingStage(manager, current, item, repoPath, workspacePath, opts, nil, "feat: previous")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}

	if len(logger.prompts) != 1 {
		t.Fatalf("expected 1 prompt log, got %d", len(logger.prompts))
	}
	if logger.prompts[0].Template != "prompt-feedback.tmpl" {
		t.Fatalf("expected prompt template, got %q", logger.prompts[0].Template)
	}
	if !strings.Contains(logger.prompts[0].Prompt, "Previous feedback") {
		t.Fatalf("expected feedback prompt to mention feedback")
	}
}

func TestRunImplementingStageRecordsEventLog(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()
	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 20, 0, 0, time.UTC)
	current, err := manager.Create("todo-events", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-events",
		Title:    "Log events",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	eventsDir := t.TempDir()
	eventLog, err := OpenEventLog(current.ID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := eventLog.Close(); err != nil {
			t.Fatalf("close event log: %v", err)
		}
	}()

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
			if err := os.WriteFile(messagePath, []byte("feat: event log"), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-event", ExitCode: 0}, nil
		},
		EventLog: eventLog,
	}

	_, err = runImplementingStage(manager, current, item, repoPath, workspacePath, opts, nil, "")
	if err != nil {
		t.Fatalf("run implementing stage: %v", err)
	}

	path := filepath.Join(eventsDir, current.ID+".jsonl")
	events := readEventLog(t, path)
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].Name != jobEventPrompt {
		t.Fatalf("expected prompt event, got %#v", events[0])
	}
	if events[1].Name != jobEventOpencodeStart {
		t.Fatalf("expected opencode start event, got %#v", events[1])
	}
	if events[2].Name != jobEventOpencodeEnd {
		t.Fatalf("expected opencode end event, got %#v", events[2])
	}
	if events[3].Name != jobEventCommitMessage {
		t.Fatalf("expected commit message event, got %#v", events[3])
	}

	var promptData map[string]string
	if err := json.Unmarshal([]byte(events[0].Data), &promptData); err != nil {
		t.Fatalf("decode prompt data: %v", err)
	}
	if promptData["purpose"] != "implement" {
		t.Fatalf("expected prompt purpose implement, got %q", promptData["purpose"])
	}
	if promptData["template"] != "prompt-implementation.tmpl" {
		t.Fatalf("expected prompt template, got %q", promptData["template"])
	}

	var opencodeData map[string]any
	if err := json.Unmarshal([]byte(events[2].Data), &opencodeData); err != nil {
		t.Fatalf("decode opencode data: %v", err)
	}
	if opencodeData["session_id"] != "oc-event" {
		t.Fatalf("expected session id oc-event, got %v", opencodeData["session_id"])
	}
	if opencodeData["exit_code"] != float64(0) {
		t.Fatalf("expected exit code 0, got %v", opencodeData["exit_code"])
	}
}

func TestRunReviewingStageLogsFeedback(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 30, 0, 0, time.UTC)
	current, err := manager.Create("todo-review-log", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-review-log",
		Title:    "Review log",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	logger := &captureLogger{}
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			if err := os.WriteFile(feedbackPath, []byte("REQUEST_CHANGES\n\nAdd tests."), 0o644); err != nil {
				return OpencodeRunResult{}, err
			}
			return OpencodeRunResult{SessionID: "oc-review", ExitCode: 0}, nil
		},
		Logger: logger,
	}

	_, err = runReviewingStage(manager, current, item, repoPath, workspacePath, opts, "feat: review", nil, reviewScopeStep)
	if err != nil {
		t.Fatalf("run reviewing stage: %v", err)
	}

	if len(logger.reviews) != 1 {
		t.Fatalf("expected 1 review log, got %d", len(logger.reviews))
	}
	if logger.reviews[0].Purpose != "review" {
		t.Fatalf("expected review purpose, got %q", logger.reviews[0].Purpose)
	}
	if logger.reviews[0].Feedback.Outcome != ReviewOutcomeRequestChanges {
		t.Fatalf("expected review outcome REQUEST_CHANGES, got %q", logger.reviews[0].Feedback.Outcome)
	}
	if logger.reviews[0].Feedback.Details != "Add tests." {
		t.Fatalf("expected review details, got %q", logger.reviews[0].Feedback.Details)
	}
}

func TestRunTestingStageLogsResults(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 12, 45, 0, 0, time.UTC)
	current, err := manager.Create("todo-test-log", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	logger := &captureLogger{}
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Job: config.Job{TestCommands: []string{"go test ./..."}}}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return []TestCommandResult{{Command: "go test ./...", ExitCode: 1}}, nil
		},
		Logger: logger,
	}

	_, err = runTestingStage(manager, current, repoPath, workspacePath, opts)
	if err != nil {
		t.Fatalf("run testing stage: %v", err)
	}

	if len(logger.tests) != 1 {
		t.Fatalf("expected 1 test log, got %d", len(logger.tests))
	}
	if len(logger.tests[0].Results) != 1 {
		t.Fatalf("expected 1 test result, got %d", len(logger.tests[0].Results))
	}
	if logger.tests[0].Results[0].Command != "go test ./..." {
		t.Fatalf("expected test command, got %q", logger.tests[0].Results[0].Command)
	}
}

func TestRunCommittingStageLogsFinalMessage(t *testing.T) {
	stateDir := t.TempDir()
	repoPath := t.TempDir()
	workspacePath := t.TempDir()

	manager, err := Open(repoPath, OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2026, 1, 12, 13, 0, 0, 0, time.UTC)
	current, err := manager.Create("todo-commit-log", startedAt)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	item := todo.Todo{
		ID:       "todo-commit-log",
		Title:    "Commit log",
		Type:     todo.TypeTask,
		Priority: todo.PriorityLow,
	}

	logger := &captureLogger{}
	opts := RunOptions{
		Now: func() time.Time {
			return startedAt
		},
		UpdateStale: func(string) error {
			return nil
		},
		OpencodeTranscripts: func(string, []OpencodeSession) ([]OpencodeTranscript, error) {
			return nil, nil
		},
		CommitIDAt: func(string, string) (string, error) {
			return "commit-999", nil
		},
		Commit: func(string, string) error {
			return nil
		},
		Logger: logger,
	}

	_, err = runCommittingStage(CommittingStageOptions{
		Manager:       manager,
		Current:       current,
		Item:          item,
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		RunOptions:    opts,
		Result:        &RunResult{},
		CommitMessage: "feat: final log",
	})
	if err != nil {
		t.Fatalf("run committing stage: %v", err)
	}

	if len(logger.commits) != 1 {
		t.Fatalf("expected 1 commit log, got %d", len(logger.commits))
	}
	if logger.commits[0].Label != "Final" {
		t.Fatalf("expected final commit label, got %q", logger.commits[0].Label)
	}
	if !strings.Contains(logger.commits[0].Message, "feat: final log") {
		t.Fatalf("expected commit message contents, got %q", logger.commits[0].Message)
	}
}

func stripANSI(value string) string {
	ansi := regexp.MustCompile("\\x1b\\[[0-9;]*m")
	return ansi.ReplaceAllString(value, "")
}
