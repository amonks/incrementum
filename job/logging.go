package job

import (
	"fmt"
	"io"
	"strings"

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
		formatMarkdownBody(entry.Prompt, subdocumentIndent),
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
	logger.writeBlock(
		formatLogLabel(logger.headerStyle.Render(label), documentIndent),
		formatCommitMessageBody(entry.Message, subdocumentIndent, entry.Preformatted),
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
	results := make([]testResultLog, 0, len(entry.Results))
	for _, result := range entry.Results {
		results = append(results, testResultLog{
			Command:  result.Command,
			ExitCode: result.ExitCode,
			Output:   result.Output,
		})
	}
	logger.writeBlock(formatTestLogBody(results))
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
		if strings.TrimSpace(body) == "-" {
			return IndentBlock(body, indent)
		}
		width := lineWidth - indent
		if width < 1 {
			width = 1
		}
		rendered := RenderMarkdown(body, width)
		if strings.TrimSpace(rendered) == "" {
			return IndentBlock("-", indent)
		}
		return IndentBlock(rendered, indent)
	}
	return IndentBlock(body, indent)
}

func formatCommitMessageBody(body string, indent int, preformatted bool) string {
	return formatMarkdownBlock(body, indent, preformatted)
}

// Markdown hard line breaks use two trailing spaces.
const markdownHardBreakPadding = 2

func preserveMarkdownLineBreaks(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = line + "  "
	}
	return strings.Join(lines, "\n")
}

func formatMarkdownBody(body string, indent int) string {
	return formatMarkdownBlock(body, indent, false)
}

func formatMarkdownBlock(body string, indent int, preformatted bool) string {
	body = strings.TrimRight(body, "\r\n")
	if strings.TrimSpace(body) == "" {
		return IndentBlock("-", indent)
	}
	width := lineWidth - indent
	if width < 1 {
		width = 1
	}
	renderWidth := width
	if preformatted {
		body = preserveMarkdownLineBreaks(body)
		renderWidth = width + markdownHardBreakPadding
	}
	rendered := RenderMarkdown(body, renderWidth)
	if strings.TrimSpace(rendered) == "" {
		return IndentBlock("-", indent)
	}
	return IndentBlock(rendered, indent)
}

type testResultLog struct {
	Command  string
	ExitCode int
	Output   string
}

func formatTestLogBody(results []testResultLog) string {
	if len(results) == 0 {
		return formatLogBody("-", documentIndent, false)
	}
	var builder strings.Builder
	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n\n")
		}
		fmt.Fprintf(&builder, "Command: %s\nExit Code: %d\nOutput:\n", result.Command, result.ExitCode)
		output := normalizeLogBody(result.Output)
		builder.WriteString(IndentBlock(output, subdocumentIndent-documentIndent))
	}
	return formatLogBody(builder.String(), documentIndent, false)
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
