package job

import "testing"

func TestFormatTestFeedback(t *testing.T) {
	results := []TestCommandResult{
		{Command: "go test ./...", ExitCode: 1},
		{Command: "golangci-lint run", ExitCode: 2},
	}

	output := FormatTestFeedback(results)
	if output == "" {
		t.Fatalf("expected feedback output, got empty string")
	}

	expected := "| Command | Exit Code |\n| --- | --- |\n| go test ./... | 1 |\n| golangci-lint run | 2 |"
	if output != expected {
		t.Fatalf("expected %q, got %q", expected, output)
	}
}
