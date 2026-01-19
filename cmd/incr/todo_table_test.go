package main

import (
	"testing"

	"github.com/amonks/incrementum/todo"
)

func TestFormatTodoTablePreservesAlignmentWithANSI(t *testing.T) {
	todos := []todo.Todo{
		{
			ID:       "abc123",
			Priority: 1,
			Type:     todo.TodoType("task"),
			Status:   todo.StatusOpen,
			Title:    "First item",
		},
		{
			ID:       "abd456",
			Priority: 2,
			Type:     todo.TodoType("bug"),
			Status:   todo.StatusInProgress,
			Title:    "Second item",
		},
	}

	plain := formatTodoTable(todos, func(id string, prefix int) string { return id })
	ansi := formatTodoTable(todos, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	})

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}
