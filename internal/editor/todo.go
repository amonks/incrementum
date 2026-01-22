package editor

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/amonks/incrementum/todo"
)

// TodoData represents the data used to render the TOML template.
type TodoData struct {
	// IsUpdate is true when editing an existing todo.
	IsUpdate bool
	// ID is the todo ID (only for updates).
	ID string
	// Title is the todo title.
	Title string
	// Type is the todo type (task, bug, feature).
	Type string
	// Priority is the todo priority (0-4).
	Priority int
	// Status is the todo status (only for updates).
	Status string
	// Description is the todo description.
	Description string
}

// DefaultCreateData returns TodoData with default values for creating a new todo.
func DefaultCreateData() TodoData {
	return TodoData{
		IsUpdate:    false,
		Title:       "",
		Type:        string(todo.TypeTask),
		Priority:    todo.PriorityMedium,
		Description: "",
	}
}

// DataFromTodo creates TodoData from an existing todo for editing.
func DataFromTodo(t *todo.Todo) TodoData {
	return TodoData{
		IsUpdate:    true,
		ID:          t.ID,
		Title:       t.Title,
		Type:        string(t.Type),
		Priority:    t.Priority,
		Status:      string(t.Status),
		Description: t.Description,
	}
}

var todoTemplate = template.Must(template.New("todo").Funcs(template.FuncMap{
	"description": func(s string) string {
		if s == "" {
			return ""
		}
		return s
	},
}).Parse(`title = {{ printf "%q" .Title }}
 type = {{ printf "%q" .Type }} # task, bug, feature
 priority = {{ .Priority }} # 0=critical, 1=high, 2=medium, 3=low, 4=backlog
{{- if .IsUpdate }}
 status = {{ printf "%q" .Status }} # open, in_progress, closed, done, tombstone
{{- end }}
---
{{ description .Description }}
`))

// RenderTodoTOML renders the todo data as a TOML string for editing.
func RenderTodoTOML(data TodoData) (string, error) {
	var buf bytes.Buffer
	if err := todoTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

// ParsedTodo represents the parsed result from the TOML editor output.
type ParsedTodo struct {
	Title       string  `toml:"title"`
	Type        string  `toml:"type"`
	Priority    int     `toml:"priority"`
	Status      *string `toml:"status"`
	Description string
}

// ParseTodoTOML parses the TOML content from the editor.
func ParseTodoTOML(content string) (*ParsedTodo, error) {
	frontmatter, body := splitFrontmatter(content)

	var parsed ParsedTodo
	if _, err := toml.Decode(frontmatter, &parsed); err != nil {
		return nil, fmt.Errorf("parse TOML: %w", err)
	}
	parsed.Description = strings.TrimLeft(body, "\n")
	parsed.Type = strings.ToLower(strings.TrimSpace(parsed.Type))
	if parsed.Status != nil {
		normalizedStatus := strings.ToLower(strings.TrimSpace(*parsed.Status))
		parsed.Status = &normalizedStatus
	}

	// Validate required fields
	if err := todo.ValidateTitle(parsed.Title); err != nil {
		return nil, err
	}
	if !todo.TodoType(parsed.Type).IsValid() {
		return nil, fmt.Errorf("invalid type %q: must be task, bug, or feature", parsed.Type)
	}
	if err := todo.ValidatePriority(parsed.Priority); err != nil {
		return nil, err
	}
	if parsed.Status != nil && !todo.Status(*parsed.Status).IsValid() {
		return nil, fmt.Errorf("invalid status %q: must be %s", *parsed.Status, todoValidStatusList())
	}

	return &parsed, nil
}

func splitFrontmatter(content string) (string, string) {
	content = strings.TrimLeft(content, "\n")
	if content == "" {
		return "", ""
	}

	lines := strings.Split(content, "\n")
	separatorIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			separatorIndex = i
			break
		}
	}
	if separatorIndex == -1 {
		return content, ""
	}

	frontmatter := strings.Join(lines[:separatorIndex], "\n")
	body := strings.Join(lines[separatorIndex+1:], "\n")
	return frontmatter, body
}

func createTodoTempFile() (*os.File, error) {
	return os.CreateTemp("", "ii-todo-*.md")
}

func todoValidStatusList() string {
	valid := todo.ValidStatuses()
	values := make([]string, 0, len(valid))
	for _, status := range valid {
		values = append(values, string(status))
	}
	return strings.Join(values, ", ")
}

// EditTodo opens the editor for a todo and returns the parsed result.
// For create: pass nil for existing.
// For update: pass the existing todo.
func EditTodo(existing *todo.Todo) (*ParsedTodo, error) {
	var data TodoData
	if existing == nil {
		data = DefaultCreateData()
	} else {
		data = DataFromTodo(existing)
	}
	return EditTodoWithData(data)
}

// EditTodoWithData opens the editor with pre-populated data and returns the parsed result.
func EditTodoWithData(data TodoData) (*ParsedTodo, error) {
	content, err := RenderTodoTOML(data)
	if err != nil {
		return nil, err
	}

	// Create temp file
	tmpfile, err := createTodoTempFile()
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpfile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpfile.WriteString(content); err != nil {
		tmpfile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Open editor
	if err := Edit(tmpPath); err != nil {
		return nil, err
	}

	// Read the edited content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read edited file: %w", err)
	}

	return ParseTodoTOML(string(edited))
}

// ToCreateOptions converts a ParsedTodo to todo.CreateOptions.
func (p *ParsedTodo) ToCreateOptions() todo.CreateOptions {
	return todo.CreateOptions{
		Type:        todo.TodoType(p.Type),
		Priority:    todo.PriorityPtr(p.Priority),
		Description: p.Description,
	}
}

// ToUpdateOptions converts a ParsedTodo to todo.UpdateOptions.
func (p *ParsedTodo) ToUpdateOptions() todo.UpdateOptions {
	opts := todo.UpdateOptions{
		Title:       &p.Title,
		Description: &p.Description,
	}

	typ := todo.TodoType(p.Type)
	opts.Type = &typ
	opts.Priority = &p.Priority

	if p.Status != nil {
		status := todo.Status(*p.Status)
		opts.Status = &status
	}
	return opts
}
