package job

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/amonks/incrementum/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

// Logger captures structured job log entries.
type Logger interface {
	Prompt(PromptLog)
	CommitMessage(CommitMessageLog)
	Review(ReviewLog)
	Tests(TestLog)
}

// PromptLog captures prompt details.
type PromptLog struct {
	Purpose  string
	Template string
	Prompt   string
}

// CommitMessageLog captures commit message text.
type CommitMessageLog struct {
	Label   string
	Message string
}

// ReviewLog captures review feedback.
type ReviewLog struct {
	Purpose  string
	Feedback ReviewFeedback
}

// TestLog captures test command results.
type TestLog struct {
	Results []TestCommandResult
}

type noopLogger struct{}

func (noopLogger) Prompt(PromptLog)               {}
func (noopLogger) CommitMessage(CommitMessageLog) {}
func (noopLogger) Review(ReviewLog)               {}
func (noopLogger) Tests(TestLog)                  {}

// ConsoleLogger writes formatted log output.
type ConsoleLogger struct {
	writer      io.Writer
	headerStyle lipgloss.Style
	subtleStyle lipgloss.Style
	started     bool
}

// NewConsoleLogger builds a styled logger for interactive output.
func NewConsoleLogger(writer io.Writer) *ConsoleLogger {
	if writer == nil {
		writer = io.Discard
	}
	return &ConsoleLogger{
		writer:      writer,
		headerStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")),
		subtleStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

// Prompt logs a prompt entry.
func (logger *ConsoleLogger) Prompt(entry PromptLog) {
	if logger == nil {
		return
	}
	label := logger.headerStyle.Render(fmt.Sprintf("Prompt: %s", entry.Purpose))
	if entry.Template != "" {
		label += " " + logger.subtleStyle.Render("("+entry.Template+")")
	}
	logger.writeSection(label, entry.Prompt)
}

// CommitMessage logs a commit message entry.
func (logger *ConsoleLogger) CommitMessage(entry CommitMessageLog) {
	if logger == nil {
		return
	}
	label := logger.headerStyle.Render(fmt.Sprintf("Commit Message: %s", entry.Label))
	logger.writeSection(label, entry.Message)
}

// Review logs review feedback.
func (logger *ConsoleLogger) Review(entry ReviewLog) {
	if logger == nil {
		return
	}
	label := logger.headerStyle.Render(fmt.Sprintf("Review Outcome: %s", entry.Feedback.Outcome))
	if entry.Purpose != "" {
		label += " " + logger.subtleStyle.Render("("+entry.Purpose+")")
	}
	logger.writeSection(label, entry.Feedback.Details)
}

// Tests logs test results.
func (logger *ConsoleLogger) Tests(entry TestLog) {
	if logger == nil {
		return
	}
	label := logger.headerStyle.Render("Test Results")
	if len(entry.Results) == 0 {
		logger.writeSection(label, "-")
		return
	}
	rows := make([][]string, 0, len(entry.Results))
	for _, result := range entry.Results {
		rows = append(rows, []string{result.Command, strconv.Itoa(result.ExitCode)})
	}
	body := ui.FormatTable([]string{"Command", "Exit Code"}, rows)
	logger.writeSection(label, body)
}

func (logger *ConsoleLogger) writeSection(label, body string) {
	if logger.started {
		fmt.Fprintln(logger.writer)
	}
	logger.started = true
	fmt.Fprintln(logger.writer, label)
	fmt.Fprintln(logger.writer, indentBlock(normalizeLogBody(body), 2))
}

func normalizeLogBody(value string) string {
	value = strings.TrimRight(value, "\r\n")
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func indentBlock(value string, spaces int) string {
	value = strings.TrimRight(value, "\r\n")
	if spaces <= 0 {
		return value
	}
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
