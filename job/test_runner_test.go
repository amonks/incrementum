package job

import "testing"

func TestRunTestCommandsCapturesExitCodes(t *testing.T) {
	results, err := RunTestCommands(t.TempDir(), []string{"true", "false"})
	if err != nil {
		t.Fatalf("run test commands: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Command != "true" || results[0].ExitCode != 0 {
		t.Fatalf("expected first result to be true/0, got %+v", results[0])
	}
	if results[1].Command != "false" || results[1].ExitCode != 1 {
		t.Fatalf("expected second result to be false/1, got %+v", results[1])
	}
}

func TestRunTestCommandsRejectsBlankCommand(t *testing.T) {
	_, err := RunTestCommands(t.TempDir(), []string{"  "})
	if err == nil {
		t.Fatalf("expected error for blank command")
	}
}
