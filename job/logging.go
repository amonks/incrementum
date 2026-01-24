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
	Purpose    string
	Template   string
	Prompt     string
	Transcript string
}

// CommitMessageLog captures commit message text.
type CommitMessageLog struct {
	Label        string
	Message      string
	Preformatted bool
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
	}
}

// ResetSpacing clears entry spacing so the next log block is adjacent.
func (logger *ConsoleLogger) ResetSpacing() {
	if logger == nil {
		return
	}
	logger.started = false
}

// Prompt logs a prompt entry.
func (logger *ConsoleLogger) Prompt(entry PromptLog) {
	if logger == nil {
		return
	}
	label := promptLabel(entry.Purpose)
	lines := []string{
		formatLogLabel(logger.headerStyle.Render(label), documentIndent),
		formatLogBody(entry.Prompt, subdocumentIndent, true),
	}
	if strings.TrimSpace(entry.Transcript) != "" {
		lines = append(lines,
			formatLogLabel(logger.headerStyle.Render("Opencode transcript:"), documentIndent),
			formatLogBody(entry.Transcript, subdocumentIndent, true),
		)
	}
	logger.writeBlock(lines...)
}

// CommitMessage logs a commit message entry.
func (logger *ConsoleLogger) CommitMessage(entry CommitMessageLog) {
	if logger == nil {
		return
	}
	label := commitMessageLabel(entry.Label)
	wrap := !entry.Preformatted
	logger.writeBlock(
		formatLogLabel(logger.headerStyle.Render(label), documentIndent),
		formatLogBody(entry.Message, subdocumentIndent, wrap),
	)
}

// Review logs review feedback.
func (logger *ConsoleLogger) Review(entry ReviewLog) {
	if logger == nil {
		return
	}
	label := reviewLabel(entry.Purpose)
	logger.writeBlock(
		formatLogLabel(logger.headerStyle.Render(label), documentIndent),
		formatLogBody(entry.Feedback.Details, subdocumentIndent, true),
	)
}

// Tests logs test results.
func (logger *ConsoleLogger) Tests(entry TestLog) {
	if logger == nil {
		return
	}
	if len(entry.Results) == 0 {
		logger.writeBlock(formatTestLogBody(nil))
		return
	}
	rows := make([][]string, 0, len(entry.Results))
	for _, result := range entry.Results {
		rows = append(rows, []string{result.Command, strconv.Itoa(result.ExitCode)})
	}
	logger.writeBlock(formatTestLogBody(rows))
}

func (logger *ConsoleLogger) writeBlock(lines ...string) {
	if len(lines) == 0 {
		return
	}
	if logger.started {
		fmt.Fprintln(logger.writer)
	}
	logger.started = true
	for _, line := range lines {
		fmt.Fprintln(logger.writer, line)
	}
}

func normalizeLogBody(value string) string {
	value = strings.TrimRight(value, "\r\n")
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func formatLogLabel(label string, indent int) string {
	if strings.TrimSpace(label) == "" {
		return ""
	}
	return IndentBlock(label, indent)
}

func formatLogBody(body string, indent int, wrap bool) string {
	body = normalizeLogBody(body)
	if wrap {
		return ReflowIndentedText(body, lineWidth, indent)
	}
	return IndentBlock(body, indent)
}

func formatTestLogBody(rows [][]string) string {
	if len(rows) == 0 {
		return formatLogBody("-", documentIndent, false)
	}
	body := ui.FormatTable([]string{"Command", "Exit Code"}, rows)
	return formatLogBody(body, documentIndent, false)
}

func promptLabel(purpose string) string {
	switch purpose {
	case "implement":
		return "Implementation prompt:"
	case "review":
		return "Code review prompt:"
	case "project-review":
		return "Project review prompt:"
	default:
		return "Prompt:"
	}
}

func reviewLabel(purpose string) string {
	switch purpose {
	case "review":
		return "Code review result:"
	case "project-review":
		return "Project review result:"
	default:
		return "Review result:"
	}
}

func commitMessageLabel(label string) string {
	if strings.TrimSpace(label) == "" {
		return "Commit message:"
	}
	return fmt.Sprintf("%s commit message:", label)
}

// StageMessage returns the standard log message for a stage transition.
func StageMessage(stage Stage) string {
	switch stage {
	case StageImplementing:
		return "Running implementation prompt:"
	case StageTesting:
		return "Implementation prompt complete; running tests:"
	case StageReviewing:
		return "Tests passed; doing code review:"
	case StageCommitting:
		return "Review complete; committing changes:"
	default:
		return fmt.Sprintf("Stage: %s", stage)
	}
}
