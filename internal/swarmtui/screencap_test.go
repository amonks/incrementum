package swarmtui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	screencapWidth  = 100
	screencapHeight = 26
)

func TestSwarmTUIScreencapTodos(t *testing.T) {
	useASCIIRenderer(t)

	m := buildTodoScreencapModel()
	assertScreencap(t, "todos.txt", m.View())
}

func TestSwarmTUIScreencapJobs(t *testing.T) {
	useASCIIRenderer(t)

	m := buildJobsScreencapModel()
	assertScreencap(t, "jobs.txt", m.View())
}

func buildTodoScreencapModel() model {
	now := time.Date(2026, 1, 20, 10, 30, 0, 0, time.UTC)
	started := now.Add(-3 * time.Hour)

	m := newModel(context.Background(), nil)
	m.width = screencapWidth
	m.height = screencapHeight
	m.resize()

	todos := []todo.Todo{
		{
			ID:          "todo-1234",
			Title:       "Fix TUI navigation",
			Description: "Ensure lists move with arrows and j/k.",
			Status:      todo.StatusInProgress,
			Type:        todo.TypeTask,
			Priority:    todo.PriorityHigh,
			CreatedAt:   now.Add(-48 * time.Hour),
			UpdatedAt:   now.Add(-24 * time.Hour),
			StartedAt:   &started,
		},
		{
			ID:       "todo-9876",
			Title:    "Add screencap tests",
			Status:   todo.StatusOpen,
			Type:     todo.TypeFeature,
			Priority: todo.PriorityMedium,
		},
	}

	m.handleTodosLoaded(todosLoadedMsg{todos: todos})
	m.todoList.Select(0)
	m.updateTodoSelection()
	return m
}

func buildJobsScreencapModel() model {
	m := newModel(context.Background(), nil)
	m.width = screencapWidth
	m.height = screencapHeight
	m.resize()

	jobs := []job.Job{
		{
			ID:     "job-202",
			TodoID: "todo-1234",
			Status: job.StatusActive,
			Stage:  job.StageImplementing,
		},
		{
			ID:     "job-203",
			TodoID: "todo-9876",
			Status: job.StatusCompleted,
			Stage:  job.StageCommitting,
		},
	}

	updated, _ := m.handleJobsLoaded(jobsLoadedMsg{jobs: jobs})
	m = updated.(model)
	m.activeTab = tabJobs
	m.jobList.Select(0)
	m.updateJobSelection()
	return m
}

func useASCIIRenderer(t *testing.T) {
	originalProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(originalProfile)
	})
}

func assertScreencap(t *testing.T, name, content string) {
	t.Helper()
	content = normalizeScreencap(content)
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_SCREENCAP") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create screencap dir: %v", err)
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write screencap: %v", err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read screencap: %v", err)
	}
	expected := normalizeScreencap(string(data))
	if content != expected {
		t.Fatalf("screencap mismatch for %s\n--- expected\n%s\n--- got\n%s", name, expected, content)
	}
}

func normalizeScreencap(value string) string {
	value = strings.TrimRight(value, "\n")
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}
