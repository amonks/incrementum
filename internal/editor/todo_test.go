package editor

import (
	"strings"
	"testing"

	"github.com/amonks/incrementum/todo"
)

func TestRenderTodoTOML_Create(t *testing.T) {
	data := DefaultCreateData()
	content, err := RenderTodoTOML(data)
	if err != nil {
		t.Fatalf("RenderTodoTOML failed: %v", err)
	}

	// Check required elements are present
	if !strings.Contains(content, `title = ""`) {
		t.Error("expected empty title")
	}
	if !strings.Contains(content, `type = "task"`) {
		t.Error("expected default type 'task'")
	}
	if !strings.Contains(content, "priority = 2") {
		t.Error("expected default priority 2")
	}
	if !strings.Contains(content, "description = ") {
		t.Error("expected description field")
	}

	// Should not have status field for create
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "status = ") {
			t.Error("status should not be present for create")
		}
	}

	// Should have commented optional fields
	if !strings.Contains(content, "# design = ") {
		t.Error("expected commented design field")
	}
}

func TestRenderTodoTOML_Update(t *testing.T) {
	existing := &todo.Todo{
		ID:                 "abc12345",
		Title:              "Test Todo",
		Type:               todo.TypeFeature,
		Priority:           todo.PriorityHigh,
		Status:             todo.StatusInProgress,
		Description:        "A test description",
		Design:             "Some design notes",
		AcceptanceCriteria: "",
		Notes:              "Some notes",
	}
	data := DataFromTodo(existing)
	content, err := RenderTodoTOML(data)
	if err != nil {
		t.Fatalf("RenderTodoTOML failed: %v", err)
	}

	// Check fields are present with values
	if !strings.Contains(content, `title = "Test Todo"`) {
		t.Error("expected title to be set")
	}
	if !strings.Contains(content, `type = "feature"`) {
		t.Error("expected type to be feature")
	}
	if !strings.Contains(content, "priority = 1") {
		t.Error("expected priority to be 1 (high)")
	}
	if !strings.Contains(content, `status = "in_progress"`) {
		t.Error("expected status to be in_progress")
	}
	if !strings.Contains(content, "A test description") {
		t.Error("expected description content")
	}
	if !strings.Contains(content, "Some design notes") {
		t.Error("expected design content")
	}
}

func TestParseTodoTOML(t *testing.T) {
	content := `
title = "My Todo"
type = "bug"
priority = 0
status = "open"

description = """
This is a description
with multiple lines
"""

design = """
Design notes here
"""
`

	parsed, err := ParseTodoTOML(content)
	if err != nil {
		t.Fatalf("ParseTodoTOML failed: %v", err)
	}

	if parsed.Title != "My Todo" {
		t.Errorf("expected title 'My Todo', got %q", parsed.Title)
	}
	if parsed.Type != "bug" {
		t.Errorf("expected type 'bug', got %q", parsed.Type)
	}
	if parsed.Priority != 0 {
		t.Errorf("expected priority 0, got %d", parsed.Priority)
	}
	if parsed.Status == nil || *parsed.Status != "open" {
		t.Errorf("expected status 'open', got %v", parsed.Status)
	}
	if !strings.Contains(parsed.Description, "multiple lines") {
		t.Errorf("expected description with multiple lines, got %q", parsed.Description)
	}
	if parsed.Design == nil || !strings.Contains(*parsed.Design, "Design notes") {
		t.Errorf("expected design notes, got %v", parsed.Design)
	}
}

func TestParseTodoTOML_Validation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing title",
			content: `type = "task"`,
			wantErr: "title is required",
		},
		{
			name:    "invalid type",
			content: `title = "test"` + "\n" + `type = "invalid"`,
			wantErr: "invalid type",
		},
		{
			name: "invalid priority",
			content: `title = "test"
type = "task"
priority = 10`,
			wantErr: "priority",
		},
		{
			name: "invalid status",
			content: `title = "test"
type = "task"
priority = 2
status = "bad"`,
			wantErr: "invalid status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTodoTOML(tt.content)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestToCreateOptions(t *testing.T) {
	parsed := &ParsedTodo{
		Title:       "Test",
		Type:        "feature",
		Priority:    1,
		Description: "desc",
	}

	opts := parsed.ToCreateOptions()

	if opts.Type != todo.TypeFeature {
		t.Errorf("expected type feature, got %v", opts.Type)
	}
	if opts.Priority != 1 {
		t.Errorf("expected priority 1, got %d", opts.Priority)
	}
	if opts.Description != "desc" {
		t.Errorf("expected description 'desc', got %q", opts.Description)
	}
}

func TestToUpdateOptions(t *testing.T) {
	status := "in_progress"
	design := "design"
	parsed := &ParsedTodo{
		Title:       "Test",
		Type:        "bug",
		Priority:    2,
		Status:      &status,
		Description: "desc",
		Design:      &design,
	}

	opts := parsed.ToUpdateOptions()

	if opts.Title == nil || *opts.Title != "Test" {
		t.Errorf("expected title 'Test', got %v", opts.Title)
	}
	if opts.Type == nil || *opts.Type != todo.TypeBug {
		t.Errorf("expected type bug, got %v", opts.Type)
	}
	if opts.Priority == nil || *opts.Priority != 2 {
		t.Errorf("expected priority 2, got %v", opts.Priority)
	}
	if opts.Status == nil || *opts.Status != todo.StatusInProgress {
		t.Errorf("expected status in_progress, got %v", opts.Status)
	}
	if opts.Design == nil || *opts.Design != "design" {
		t.Errorf("expected design 'design', got %v", opts.Design)
	}
}
