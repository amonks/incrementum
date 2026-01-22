package job

import (
	"fmt"
	"strings"
)

// TestCommandResult captures a test command execution result.
type TestCommandResult struct {
	Command  string
	ExitCode int
}

// FormatTestFeedback builds a markdown table for failed test commands.
func FormatTestFeedback(results []TestCommandResult) string {
	if len(results) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("| Command | Exit Code |\n| --- | --- |")
	for _, result := range results {
		fmt.Fprintf(&builder, "\n| %s | %d |", escapeMarkdownCell(result.Command), result.ExitCode)
	}

	return builder.String()
}

func escapeMarkdownCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
