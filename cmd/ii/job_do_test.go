package main

import (
	"strings"
	"testing"

	jobpkg "github.com/amonks/incrementum/job"
)

func TestReflowJobTextPreservesParagraphs(t *testing.T) {
	input := "First paragraph line one.\nSecond line stays with paragraph.\n\nSecond paragraph follows."
	output := reflowJobText(input, 80)

	if output == "-" {
		t.Fatalf("expected non-empty output, got %q", output)
	}
	if output == input {
		t.Fatalf("expected reflowing to normalize whitespace, got %q", output)
	}
	if !strings.Contains(output, "\n\n") {
		t.Fatalf("expected paragraph break preserved, got %q", output)
	}
	if !strings.Contains(output, "First paragraph line one. Second line stays with paragraph.") {
		t.Fatalf("expected paragraph to reflow, got %q", output)
	}
}

func TestFormatCommitMessagesOutputPreservesIndentation(t *testing.T) {
	entries := []jobpkg.CommitLogEntry{{
		ID:      "commit-123",
		Message: "Summary line\n\nHere is a generated commit message:\n\n    Body line\n\nThis commit is a step towards implementing this todo:\n\n    ID: todo-1",
	}}

	output := formatCommitMessagesOutput(entries)
	if !strings.Contains(output, "Commit messages:") {
		t.Fatalf("expected header, got %q", output)
	}
	if !strings.Contains(output, "Commit commit-123:") {
		t.Fatalf("expected commit id label, got %q", output)
	}
	if !strings.Contains(output, "\nSummary line\n") {
		t.Fatalf("expected summary line without indent, got %q", output)
	}
	if strings.Contains(output, "\n        Summary line") {
		t.Fatalf("expected summary line to avoid extra indentation, got %q", output)
	}
	if !strings.Contains(output, "\n    ID: todo-1") {
		t.Fatalf("expected commit message indentation preserved, got %q", output)
	}
}

func TestStageMessageUsesCodeReviewLabel(t *testing.T) {
	message := stageMessage(jobpkg.StageReviewing)
	if message != "Tests passed; doing code review:" {
		t.Fatalf("expected code review stage message, got %q", message)
	}
}
