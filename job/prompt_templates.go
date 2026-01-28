package job

import "path/filepath"

// PromptTemplateVariable documents a template variable and its Go type.
type PromptTemplateVariable struct {
	Name string
	Type string
}

// PromptTemplateInfo documents a default prompt template.
type PromptTemplateInfo struct {
	Name      string
	Variables []PromptTemplateVariable
}

// PromptOverridePath returns the override location for a template name.
func PromptOverridePath(name string) string {
	return filepath.Join(promptOverrideDir, name)
}

// DefaultPromptTemplateInfo lists the bundled prompt templates.
func DefaultPromptTemplateInfo() []PromptTemplateInfo {
	variables := promptTemplateVariables()
	return []PromptTemplateInfo{
		{Name: "prompt-implementation.tmpl", Variables: variables},
		{Name: "prompt-feedback.tmpl", Variables: variables},
		{Name: "prompt-commit-review.tmpl", Variables: variables},
		{Name: "prompt-project-review.tmpl", Variables: variables},
	}
}

func promptTemplateVariables() []PromptTemplateVariable {
	return []PromptTemplateVariable{
		{Name: "Todo", Type: "todo.Todo"},
		{Name: "Feedback", Type: "string"},
		{Name: "Message", Type: "string"},
		{Name: "CommitLog", Type: "[]CommitLogEntry"},
		{Name: "OpencodeTranscripts", Type: "[]OpencodeTranscript"},
		{Name: "WorkspacePath", Type: "string"},
		{Name: "ReviewInstructions", Type: "string"},
		{Name: "TodoBlock", Type: "string"},
		{Name: "FeedbackBlock", Type: "string"},
		{Name: "CommitMessageBlock", Type: "string"},
	}
}
