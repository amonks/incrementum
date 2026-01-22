package job

import "testing"

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
