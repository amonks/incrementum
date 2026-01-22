package job

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestParseReviewFeedbackAccept(t *testing.T) {
	feedback, err := ParseReviewFeedback("ACCEPT\n\nextra")
	if err != nil {
		t.Fatalf("parse feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected ACCEPT, got %q", feedback.Outcome)
	}
	if feedback.Details != "" {
		t.Fatalf("expected no details, got %q", feedback.Details)
	}
}

func TestParseReviewFeedbackRequestChanges(t *testing.T) {
	contents := "REQUEST_CHANGES\n\nPlease update the tests.\nAdd coverage.\n"
	feedback, err := ParseReviewFeedback(contents)
	if err != nil {
		t.Fatalf("parse feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeRequestChanges {
		t.Fatalf("expected REQUEST_CHANGES, got %q", feedback.Outcome)
	}
	expected := "Please update the tests.\nAdd coverage."
	if feedback.Details != expected {
		t.Fatalf("expected details %q, got %q", expected, feedback.Details)
	}
}

func TestParseReviewFeedbackInvalid(t *testing.T) {
	_, err := ParseReviewFeedback("REQUEST_CHANGES\nmissing blank")
	if !errors.Is(err, ErrInvalidFeedbackFormat) {
		t.Fatalf("expected invalid feedback error, got %v", err)
	}
}

func TestReadReviewFeedbackMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-feedback")
	feedback, err := ReadReviewFeedback(path)
	if err != nil {
		t.Fatalf("read feedback: %v", err)
	}
	if feedback.Outcome != ReviewOutcomeAccept {
		t.Fatalf("expected ACCEPT, got %q", feedback.Outcome)
	}
}
