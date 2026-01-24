package job

import (
	"strings"
	"testing"
)

func TestLogSnapshotFormatsJobEvents(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-logs"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageImplementing}); err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventPrompt, promptEventData{Purpose: "implement", Prompt: "First line.\nSecond line."}); err != nil {
		t.Fatalf("append prompt event: %v", err)
	}
	if err := appendJobEvent(log, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: "feat: add logs"}); err != nil {
		t.Fatalf("append commit message event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageTesting}); err != nil {
		t.Fatalf("append test stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTests, buildTestsEventData([]TestCommandResult{{Command: "go test ./...", ExitCode: 1}})); err != nil {
		t.Fatalf("append tests event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageReviewing}); err != nil {
		t.Fatalf("append review stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventReview, reviewEventData{Purpose: "review", Outcome: ReviewOutcomeRequestChanges, Details: "Add tests."}); err != nil {
		t.Fatalf("append review event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	checks := []string{
		"Running implementation prompt:",
		"    Implementation prompt:",
		"        First line. Second line.",
		"    Draft commit message:",
		"        feat: add logs",
		"Implementation prompt complete; running tests:",
		"Command",
		"Exit Code",
		"go test ./...",
		"Tests passed; doing code review:",
		"    Code review result:",
		"        Add tests.",
	}
	for _, check := range checks {
		if !strings.Contains(snapshot, check) {
			t.Fatalf("expected snapshot to include %q, got %q", check, snapshot)
		}
	}
}

func TestLogSnapshotHandlesLargeEvent(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-logs-large"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	largePrompt := strings.Repeat("word ", 20000)
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageImplementing}); err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventPrompt, promptEventData{Purpose: "implement", Prompt: largePrompt}); err != nil {
		t.Fatalf("append prompt event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if !strings.Contains(snapshot, "    Implementation prompt:") {
		t.Fatalf("expected prompt label, got %q", snapshot)
	}
	if !strings.Contains(snapshot, "word word word") {
		t.Fatalf("expected large prompt content, got %q", snapshot)
	}
}
