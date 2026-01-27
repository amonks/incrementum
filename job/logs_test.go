package job

import (
	"regexp"
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
	if err := appendJobEvent(log, jobEventTranscript, transcriptEventData{Purpose: "implement", Transcript: "Opencode line."}); err != nil {
		t.Fatalf("append transcript event: %v", err)
	}
	if err := appendJobEvent(log, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: "feat: add logs"}); err != nil {
		t.Fatalf("append commit message event: %v", err)
	}
	opencodeToolStart := `{"type":"message.part.updated","properties":{"part":{"id":"prt-tool","messageID":"msg-tool","type":"tool","tool":"read","state":{"status":"running","input":{"filePath":"/tmp/example.txt"}}}}}`
	if err := log.Append(Event{Data: opencodeToolStart}); err != nil {
		t.Fatalf("append opencode tool start event: %v", err)
	}
	opencodeToolEnd := `{"type":"message.part.updated","properties":{"part":{"id":"prt-tool","messageID":"msg-tool","type":"tool","tool":"read","state":{"status":"completed","input":{"filePath":"/tmp/example.txt"}}}}}`
	if err := log.Append(Event{Data: opencodeToolEnd}); err != nil {
		t.Fatalf("append opencode tool end event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageTesting}); err != nil {
		t.Fatalf("append test stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTests, buildTestsEventData([]TestCommandResult{{Command: "go test ./...", ExitCode: 1, Output: "tests failed"}})); err != nil {
		t.Fatalf("append tests event: %v", err)
	}
	if err := appendJobEvent(log, jobEventStage, stageEventData{Stage: StageReviewing}); err != nil {
		t.Fatalf("append review stage event: %v", err)
	}
	if err := appendJobEvent(log, jobEventTranscript, transcriptEventData{Purpose: "review", Transcript: "Review transcript line."}); err != nil {
		t.Fatalf("append review transcript event: %v", err)
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
		"    Opencode transcript:",
		"        Opencode line.",
		"    Draft commit message:",
		"        feat: add logs",
		"    Opencode tool start: read file '/tmp/example.txt'",
		"    Opencode tool end: read file '/tmp/example.txt'",
		"Implementation prompt complete; running tests:",
		"Command: go test ./...",
		"Exit Code: 1",
		"Output:",
		"tests failed",
		"Tests passed; doing code review:",
		"Review transcript line.",
		"    Code review result:",
		"        Add tests.",
	}
	for _, check := range checks {
		if !strings.Contains(snapshot, check) {
			t.Fatalf("expected snapshot to include %q, got %q", check, snapshot)
		}
	}
}

func TestLogSnapshotIncludesOpencodeError(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-opencode-error"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	if err := appendJobEvent(log, jobEventOpencodeError, opencodeErrorEventData{Purpose: "implement", Error: "opencode session not found"}); err != nil {
		t.Fatalf("append opencode error: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if !strings.Contains(snapshot, "Opencode implement error:") {
		t.Fatalf("expected opencode error label, got %q", snapshot)
	}
	if !strings.Contains(snapshot, "opencode session not found") {
		t.Fatalf("expected opencode error details, got %q", snapshot)
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

func TestLogSnapshotRendersMarkdownCommitMessage(t *testing.T) {
	eventsDir := t.TempDir()
	jobID := "job-logs-markdown"
	log, err := OpenEventLog(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
	}()

	message := "Summary:\n\n- First item\n- Second item\n\n```bash\necho first\necho second\n```"
	if err := appendJobEvent(log, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: message}); err != nil {
		t.Fatalf("append commit message event: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	checks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s+Summary:$`),
		regexp.MustCompile(`(?m)^\s+.*First item$`),
		regexp.MustCompile(`(?m)^\s+.*Second item$`),
		regexp.MustCompile(`(?m)^\s+echo first$`),
		regexp.MustCompile(`(?m)^\s+echo second$`),
	}
	for _, check := range checks {
		if !check.MatchString(snapshot) {
			t.Fatalf("expected markdown commit message output to match %q, got %q", check.String(), snapshot)
		}
	}
}

func TestEventFormatterAppendsOutput(t *testing.T) {
	formatter := NewEventFormatter()

	chunk, err := formatter.Append(Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"})
	if err != nil {
		t.Fatalf("append stage event: %v", err)
	}
	if !strings.Contains(chunk, "Running implementation prompt:") {
		t.Fatalf("expected stage output, got %q", chunk)
	}

	chunk, err = formatter.Append(Event{Name: "job.prompt", Data: "{\"purpose\":\"implement\",\"prompt\":\"Hello\"}"})
	if err != nil {
		t.Fatalf("append prompt event: %v", err)
	}
	if !strings.Contains(chunk, "Implementation prompt:") {
		t.Fatalf("expected prompt output, got %q", chunk)
	}

	opencodeToolStart := `{"type":"message.part.updated","properties":{"part":{"id":"prt-tool","messageID":"msg-tool","type":"tool","tool":"glob","state":{"status":"running","input":{"pattern":"**/*.go","path":"/tmp"}}}}}`
	chunk, err = formatter.Append(Event{Data: opencodeToolStart})
	if err != nil {
		t.Fatalf("append opencode tool start event: %v", err)
	}
	if !strings.Contains(chunk, "Opencode tool start: glob '") {
		t.Fatalf("expected opencode tool start output, got %q", chunk)
	}
	opencodeToolEnd := `{"type":"message.part.updated","properties":{"part":{"id":"prt-tool","messageID":"msg-tool","type":"tool","tool":"glob","state":{"status":"completed","input":{"pattern":"**/*.go","path":"/tmp"}}}}}`
	chunk, err = formatter.Append(Event{Data: opencodeToolEnd})
	if err != nil {
		t.Fatalf("append opencode tool end event: %v", err)
	}
	if !strings.Contains(chunk, "Opencode tool end: glob '") {
		t.Fatalf("expected opencode tool end output, got %q", chunk)
	}

	chunk, err = formatter.Append(Event{Name: "job.opencode.error", Data: "{\"purpose\":\"implement\",\"error\":\"opencode session not found\"}"})
	if err != nil {
		t.Fatalf("append opencode error: %v", err)
	}
	if !strings.Contains(chunk, "Opencode implement error:") {
		t.Fatalf("expected opencode error label, got %q", chunk)
	}
	if !strings.Contains(chunk, "opencode session not found") {
		t.Fatalf("expected opencode error details, got %q", chunk)
	}
}

func TestEventFormatterRendersOpencodeMessages(t *testing.T) {
	formatter := NewEventFormatter()

	userMessage := `{"type":"message.updated","properties":{"info":{"id":"msg-user","role":"user"}}}`
	userPrompt := `{"type":"message.part.updated","properties":{"part":{"id":"prt-user","messageID":"msg-user","type":"text","text":"Prompt line."}}}`
	if _, err := formatter.Append(Event{Data: userMessage}); err != nil {
		t.Fatalf("append user message event: %v", err)
	}
	chunk, err := formatter.Append(Event{Data: userPrompt})
	if err != nil {
		t.Fatalf("append user prompt event: %v", err)
	}
	if !strings.Contains(chunk, "Opencode prompt:") {
		t.Fatalf("expected opencode prompt label, got %q", chunk)
	}
	if !strings.Contains(chunk, "Prompt line.") {
		t.Fatalf("expected opencode prompt text, got %q", chunk)
	}

	assistantMessage := `{"type":"message.updated","properties":{"info":{"id":"msg-assistant","role":"assistant"}}}`
	assistantText := `{"type":"message.part.updated","properties":{"part":{"id":"prt-assistant","messageID":"msg-assistant","type":"text","text":"Response line."}}}`
	assistantThinking := `{"type":"message.part.updated","properties":{"part":{"id":"prt-reason","messageID":"msg-assistant","type":"reasoning","text":"Thinking line."}}}`
	assistantComplete := `{"type":"message.updated","properties":{"info":{"id":"msg-assistant","role":"assistant","time":{"completed":1}}}}`
	if _, err := formatter.Append(Event{Data: assistantMessage}); err != nil {
		t.Fatalf("append assistant message event: %v", err)
	}
	if _, err := formatter.Append(Event{Data: assistantText}); err != nil {
		t.Fatalf("append assistant text event: %v", err)
	}
	if _, err := formatter.Append(Event{Data: assistantThinking}); err != nil {
		t.Fatalf("append assistant thinking event: %v", err)
	}
	chunk, err = formatter.Append(Event{Data: assistantComplete})
	if err != nil {
		t.Fatalf("append assistant complete event: %v", err)
	}
	if !strings.Contains(chunk, "Opencode thinking:") {
		t.Fatalf("expected opencode thinking label, got %q", chunk)
	}
	if !strings.Contains(chunk, "Thinking line.") {
		t.Fatalf("expected opencode thinking text, got %q", chunk)
	}
	if !strings.Contains(chunk, "Opencode response:") {
		t.Fatalf("expected opencode response label, got %q", chunk)
	}
	if !strings.Contains(chunk, "Response line.") {
		t.Fatalf("expected opencode response text, got %q", chunk)
	}
}

func TestEventFormatterRendersOpencodePromptMarkdown(t *testing.T) {
	formatter := NewEventFormatter()

	userMessage := `{"type":"message.updated","properties":{"info":{"id":"msg-user","role":"user"}}}`
	userPrompt := `{"type":"message.part.updated","properties":{"part":{"id":"prt-user","messageID":"msg-user","type":"text","text":"Checklist:\n\n- First item\n- Second item"}}}`
	if _, err := formatter.Append(Event{Data: userMessage}); err != nil {
		t.Fatalf("append user message event: %v", err)
	}
	chunk, err := formatter.Append(Event{Data: userPrompt})
	if err != nil {
		t.Fatalf("append user prompt event: %v", err)
	}

	checks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s+Checklist:$`),
		regexp.MustCompile(`(?m)^\s+.*First item$`),
		regexp.MustCompile(`(?m)^\s+.*Second item$`),
	}
	for _, check := range checks {
		if !check.MatchString(chunk) {
			t.Fatalf("expected opencode markdown output to match %q, got %q", check.String(), chunk)
		}
	}
}

func TestEventFormatterFallsBackOnMalformedOpencodeMessage(t *testing.T) {
	formatter := NewEventFormatter()

	malformed := `{"type":"message.updated","properties":"nope"}`
	chunk, err := formatter.Append(Event{Data: malformed})
	if err != nil {
		t.Fatalf("append malformed opencode event: %v", err)
	}
	if !strings.Contains(chunk, "Opencode event (message.updated):") {
		t.Fatalf("expected fallback opencode label, got %q", chunk)
	}
	if !strings.Contains(chunk, `"type":"message.updated"`) {
		t.Fatalf("expected raw opencode payload, got %q", chunk)
	}
}
