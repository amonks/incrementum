package main

import (
	"fmt"
	"strings"
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

	prefixLengths := todoIDPrefixLengths(todos)
	plain := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string { return id })
	ansi := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	})

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatTodoTableUsesProvidedPrefixLengths(t *testing.T) {
	todos := []todo.Todo{
		{
			ID:       "r1234567",
			Priority: 2,
			Type:     todo.TodoType("task"),
			Status:   todo.StatusOpen,
			Title:    "Only listed",
		},
	}

	prefixLengths := map[string]int{"r1234567": 2}
	output := formatTodoTable(todos, prefixLengths, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	})

	if !strings.Contains(output, "r1234567:2") {
		t.Fatalf("expected table to use provided prefix length, got:\n%s", output)
	}
}
