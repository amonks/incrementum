package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amonks/incrementum/todo"
)

func TestLoadPrompt_UsesOverride(t *testing.T) {
	repoPath := t.TempDir()
	promptDir := filepath.Join(repoPath, ".incrementum", "templates")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}

	override := "override content"
	overridePath := filepath.Join(promptDir, "prompt-implementation.tmpl")
	if err := os.WriteFile(overridePath, []byte(override), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	loaded, err := LoadPrompt(repoPath, "prompt-implementation.tmpl")
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}

	if strings.TrimSpace(loaded) != override {
		t.Fatalf("expected override content, got %q", loaded)
	}
}

func TestLoadPrompt_UsesEmbeddedDefault(t *testing.T) {
	repoPath := t.TempDir()

	loaded, err := LoadPrompt(repoPath, "prompt-commit-review.tmpl")
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}

	if !strings.Contains(loaded, "Review the changes") {
		t.Fatalf("expected embedded prompt, got %q", loaded)
	}
}

func TestRenderPrompt_InterpolatesFields(t *testing.T) {
	data := PromptData{
		Todo: todo.Todo{
			ID:          "todo-123",
			Title:       "Ship it",
			Description: "Do the thing",
			Type:        todo.TypeTask,
			Priority:    todo.PriorityHigh,
		},
		Feedback: "Needs more tests",
		Message:  "Add coverage",
	}

	rendered, err := RenderPrompt("{{.Todo.ID}} {{.Todo.Title}} {{.Feedback}} {{.Message}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	expected := "todo-123 Ship it Needs more tests Add coverage"
	if strings.TrimSpace(rendered) != expected {
		t.Fatalf("expected %q, got %q", expected, rendered)
	}
}

func TestRenderPrompt_InterpolatesWorkspacePath(t *testing.T) {
	data := PromptData{WorkspacePath: "/tmp/ws-123"}

	rendered, err := RenderPrompt("{{.WorkspacePath}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if strings.TrimSpace(rendered) != "/tmp/ws-123" {
		t.Fatalf("expected workspace path to render, got %q", rendered)
	}
}

func TestRenderPrompt_InterpolatesCommitLog(t *testing.T) {
	data := PromptData{
		CommitLog: []CommitLogEntry{{ID: "commit-1", Message: "feat: first change"}},
	}

	rendered, err := RenderPrompt("{{range .CommitLog}}{{.ID}} {{.Message}}{{end}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	expected := "commit-1 feat: first change"
	if strings.TrimSpace(rendered) != expected {
		t.Fatalf("expected %q, got %q", expected, rendered)
	}
}

func TestRenderPrompt_InterpolatesReviewInstructions(t *testing.T) {
	data := PromptData{ReviewInstructions: "Follow the steps."}

	rendered, err := RenderPrompt("{{.ReviewInstructions}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if strings.TrimSpace(rendered) != "Follow the steps." {
		t.Fatalf("expected review instructions to render, got %q", rendered)
	}
}

func TestRenderPrompt_InterpolatesTodoBlock(t *testing.T) {
	data := PromptData{TodoBlock: "<todo>id</todo>"}

	rendered, err := RenderPrompt("{{.TodoBlock}}", data)
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if strings.TrimSpace(rendered) != "<todo>id</todo>" {
		t.Fatalf("expected todo block to render, got %q", rendered)
	}
}

func TestRenderPrompt_RendersReviewQuestionsTemplate(t *testing.T) {
	rendered, err := RenderPrompt("{{template \"review_questions\"}}", PromptData{})
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}

	if !strings.Contains(rendered, "Does it do what the message says?") {
		t.Fatalf("expected review questions to render, got %q", rendered)
	}
}
