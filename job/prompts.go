package job

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/amonks/incrementum/todo"
)

const promptOverrideDir = ".incrementum/templates"

//go:embed templates/*.tmpl
var defaultTemplates embed.FS

// PromptData supplies values for job prompt templates.
type PromptData struct {
	Todo          todo.Todo
	Feedback      string
	Message       string
	WorkspacePath string
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
	tmpl, err := template.New("prompt").Option("missingkey=error").Parse(contents)
	if err != nil {
		return "", fmt.Errorf("parse prompt: %w", err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}
	return out.String(), nil
}
