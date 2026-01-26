package job

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
)

const promptOverrideDir = ".incrementum/templates"

//go:embed templates/*.tmpl
var defaultTemplates embed.FS

const reviewInstructionsText = "Publish your review to the file ./.incrementum-feedback\n\n" +
	"Write one of the following allcaps words as the first line:\n" +
	"- `ACCEPT` -- if the changes pass review and should be merged\n" +
	"- `ABANDON` -- if the changes are so off-base as to be a lost cause\n" +
	"- `REQUEST_CHANGES` -- if some modifications could get the changes into shape\n\n" +
	"If requesting changes, add a blank line and the details after it.\n"

const reviewQuestionsTemplate = `{{define "review_questions"}}- Does it do what the message says?
- Does it move us towards the goal in the todo?
- Is it _necessary_ for moving us towards the goal in the todo?
- Is it free of defects?
- Is the domain modeling coherent and sound?
- Are things in the right place?
- Does it include proper test coverage?
- Does it keep the relevant specs up to date?
- Does it conform to the norms of the code areas it modifies?
- Does it work?
{{end}}`

// PromptData supplies values for job prompt templates.
type PromptData struct {
	Todo                todo.Todo
	Feedback            string
	Message             string
	CommitLog           []CommitLogEntry
	OpencodeTranscripts []OpencodeTranscript
	WorkspacePath       string
	ReviewInstructions  string
	TodoBlock           string
	FeedbackBlock       string
	CommitMessageBlock  string
}

func newPromptData(item todo.Todo, feedback, message string, commitLog []CommitLogEntry, transcripts []OpencodeTranscript, workspacePath string) PromptData {
	return PromptData{
		Todo:                item,
		Feedback:            feedback,
		Message:             message,
		CommitLog:           commitLog,
		OpencodeTranscripts: transcripts,
		WorkspacePath:       workspacePath,
		ReviewInstructions:  reviewInstructionsText,
		TodoBlock:           formatTodoBlock(item),
		FeedbackBlock:       formatPromptBlock("Previous feedback", feedback),
		CommitMessageBlock:  formatPromptBlock("Commit message", message),
	}
}

func formatTodoBlock(item todo.Todo) string {
	description := internalstrings.TrimTrailingNewlines(item.Description)
	if strings.TrimSpace(description) == "" {
		description = "-"
	}
	description = ReflowIndentedText(description, lineWidth, subdocumentIndent)
	fields := []string{
		formatTodoField("ID", item.ID),
		formatTodoField("Title", item.Title),
		formatTodoField("Type", string(item.Type)),
		formatTodoField("Priority", fmt.Sprintf("%d", item.Priority)),
		"Description:",
	}
	fieldBlock := IndentBlock(strings.Join(fields, "\n"), documentIndent)
	return fmt.Sprintf("Todo\n\n%s\n%s", fieldBlock, description)
}

func formatPromptBlock(label, body string) string {
	body = internalstrings.TrimTrailingNewlines(body)
	if strings.TrimSpace(body) == "" {
		body = "-"
	}
	formatted := ReflowIndentedText(body, lineWidth, documentIndent)
	return fmt.Sprintf("%s\n\n%s", label, formatted)
}

func formatTodoField(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "-"
	}
	value = internalstrings.NormalizeWhitespace(value)
	if value == "" {
		value = "-"
	}
	return fmt.Sprintf("%s: %s", label, value)
}

// LoadPrompt loads a prompt template for the repo.
func LoadPrompt(repoPath, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("prompt name is required")
	}

	if repoPath != "" {
		overridePath := filepath.Join(repoPath, promptOverrideDir, name)
		if data, err := os.ReadFile(overridePath); err == nil {
			return string(data), nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("read prompt override: %w", err)
		}
	}

	data, err := defaultTemplates.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return "", fmt.Errorf("read default prompt: %w", err)
	}

	return string(data), nil
}

// RenderPrompt renders the prompt with provided data.
func RenderPrompt(contents string, data PromptData) (string, error) {
	tmpl, err := template.New("prompt").Option("missingkey=error").Parse(reviewQuestionsTemplate)
	if err != nil {
		return "", fmt.Errorf("parse shared prompt: %w", err)
	}

	tmpl, err = tmpl.Parse(contents)
	if err != nil {
		return "", fmt.Errorf("parse prompt: %w", err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}
	return out.String(), nil
}
