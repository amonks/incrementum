package editor

import (
	"os"
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
	if strings.Contains(content, "description =") {
		t.Error("expected description to be in body")
	}
	if !strings.Contains(content, "---") {
		t.Error("expected frontmatter separator")
	}

	// Should not have status field for create
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "status = ") {
			t.Error("status should not be present for create")
		}
	}

}

func TestRenderTodoTOML_Update(t *testing.T) {
	existing := &todo.Todo{
		ID:          "abc12345",
		Title:       "Test Todo",
		Type:        todo.TypeFeature,
		Priority:    todo.PriorityHigh,
		Status:      todo.StatusInProgress,
		Description: "A test description",
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
	if !strings.Contains(content, "tombstone") {
		t.Error("expected status comment to mention tombstone")
	}
	if strings.Contains(content, "description =") {
		t.Error("expected description to be in body")
	}
	if !strings.Contains(content, "A test description") {
		t.Error("expected description content")
	}
}

func TestParseTodoTOML(t *testing.T) {
	content := `
 title = "My Todo"
 type = "bug"
 priority = 0
 status = "done"
 ---
 This is a description
 with multiple lines
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
	if parsed.Status == nil || *parsed.Status != "done" {
		t.Errorf("expected status 'done', got %v", parsed.Status)
	}
	if strings.Contains(parsed.Description, "description =") {
		t.Errorf("expected description body without key, got %q", parsed.Description)
	}
	if !strings.Contains(parsed.Description, "multiple lines") {
		t.Errorf("expected description with multiple lines, got %q", parsed.Description)
	}
}

func TestParseTodoTOML_NormalizesCase(t *testing.T) {
	content := `title = "My Todo"
type = "BUG"
priority = 1
status = "DONE"`

	parsed, err := ParseTodoTOML(content)
	if err != nil {
		t.Fatalf("ParseTodoTOML failed: %v", err)
	}

	if parsed.Type != "bug" {
		t.Errorf("expected type 'bug', got %q", parsed.Type)
	}
	if parsed.Status == nil || *parsed.Status != "done" {
		t.Errorf("expected status 'done', got %v", parsed.Status)
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
			wantErr: "title cannot be empty",
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

func TestParseTodoTOML_InvalidStatusMentionsTombstone(t *testing.T) {
	content := `title = "test"
type = "task"
priority = 2
status = "bad"`

	_, err := ParseTodoTOML(content)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "tombstone") {
		t.Errorf("expected error to mention tombstone, got %q", err.Error())
	}
}

func TestToCreateOptions(t *testing.T) {
	parsed := &ParsedTodo{
		Title:       "Test",
		Type:        "feature",
		Priority:    1,
		Description: "description",
	}

	opts := parsed.ToCreateOptions()

	if opts.Type != todo.TypeFeature {
		t.Errorf("expected type feature, got %v", opts.Type)
	}
	if opts.Priority == nil || *opts.Priority != 1 {
		t.Errorf("expected priority 1, got %v", opts.Priority)
	}
	if opts.Description != "description" {
		t.Errorf("expected description 'description', got %q", opts.Description)
	}
}

func TestToUpdateOptions(t *testing.T) {
	status := "in_progress"
	parsed := &ParsedTodo{
		Title:       "Test",
		Type:        "bug",
		Priority:    2,
		Status:      &status,
		Description: "description",
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
}

func TestCreateTodoTempFileExtension(t *testing.T) {
	file, err := createTodoTempFile()
	if err != nil {
		t.Fatalf("createTodoTempFile failed: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(file.Name())
	})

	if !strings.HasSuffix(file.Name(), ".md") {
		t.Errorf("expected temp file to end with .md, got %q", file.Name())
	}
}
