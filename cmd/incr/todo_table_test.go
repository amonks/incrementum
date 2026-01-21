package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestFormatTodoTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	todos := []todo.Todo{
		{
			ID:        "abc123",
			Priority:  1,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusOpen,
			Title:     "First item",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "abd456",
			Priority:  2,
			Type:      todo.TodoType("bug"),
			Status:    todo.StatusInProgress,
			Title:     "Second item",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	prefixLengths := todoIDPrefixLengths(todos)
	plain := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string { return id }, now)
	ansi := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatTodoTableUsesProvidedPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	todos := []todo.Todo{
		{
			ID:        "r1234567",
			Priority:  2,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusOpen,
			Title:     "Only listed",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	prefixLengths := map[string]int{"r1234567": 2}
	output := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	}, now)

	if !strings.Contains(output, "r1234567:2") {
		t.Fatalf("expected table to use provided prefix length, got:\n%s", output)
	}
}

func TestFormatTodoTableShowsCreatedUpdatedAgo(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	todos := []todo.Todo{
		{
			ID:        "abc123",
			Priority:  2,
			Type:      todo.TodoType("task"),
			Status:    todo.StatusOpen,
			Title:     "Time check",
			CreatedAt: now.Add(-2*time.Minute - 5*time.Second),
			UpdatedAt: now.Add(-1 * time.Hour),
		},
	}

	output := formatTodoTable(todos, nil, func(id string, prefix int) string { return id }, now)

	if !strings.Contains(output, "2m5s ago") {
		t.Fatalf("expected created age in output, got:\n%s", output)
	}
	if !strings.Contains(output, "1h0m0s ago") {
		t.Fatalf("expected updated age in output, got:\n%s", output)
	}
}
