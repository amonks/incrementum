package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/todo"
)

func TestResolveDescriptionFromStdin(t *testing.T) {
	cases := []struct {
		name string
		desc string
		in   string
		want string
	}{
		{
			name: "stdin with newline",
			desc: "-",
			in:   "Hello from stdin\n",
			want: "Hello from stdin",
		},
		{
			name: "stdin without newline",
			desc: "-",
			in:   "No newline",
			want: "No newline",
		},
		{
			name: "literal description",
			desc: "Already set",
			in:   "ignored",
			want: "Already set",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveDescriptionFromStdin(tc.desc, bytes.NewBufferString(tc.in))
			if err != nil {
				t.Fatalf("resolveDescriptionFromStdin failed: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestTodoLogHighlighterUsesProvidedPrefixLengths(t *testing.T) {
	prefixLengths := map[string]int{"abc123": 4, "abd456": 3}
	highlight := todoLogHighlighter(prefixLengths, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	})

	if got := highlight("abc123"); got != "abc123:4" {
		t.Fatalf("expected abc123 to use prefix 4, got %q", got)
	}
	if got := highlight("abd456"); got != "abd456:3" {
		t.Fatalf("expected abd456 to use prefix 3, got %q", got)
	}
}

func TestTodoLogHighlighterHandlesMissingID(t *testing.T) {
	prefixLengths := map[string]int{"abc123": 4}
	highlight := todoLogHighlighter(prefixLengths, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	})

	if got := highlight("zzz999"); got != "zzz999:0" {
		t.Fatalf("expected missing id to use prefix 0, got %q", got)
	}
}

func TestShouldUseTodoUpdateEditor(t *testing.T) {
	cases := []struct {
		name           string
		hasUpdateFlags bool
		editFlag       bool
		noEditFlag     bool
		interactive    bool
		want           bool
	}{
		{
			name:           "flags set without edit",
			hasUpdateFlags: true,
			interactive:    true,
			want:           false,
		},
		{
			name:           "flags set with edit",
			hasUpdateFlags: true,
			editFlag:       true,
			interactive:    true,
			want:           true,
		},
		{
			name:           "flags set with no-edit",
			hasUpdateFlags: true,
			noEditFlag:     true,
			interactive:    true,
			want:           false,
		},
		{
			name:        "no flags interactive",
			interactive: true,
			want:        true,
		},
		{
			name: "no flags non-interactive",
			want: false,
		},
		{
			name:        "no flags with edit",
			editFlag:    true,
			interactive: false,
			want:        true,
		},
		{
			name:        "no flags with no-edit",
			noEditFlag:  true,
			interactive: true,
			want:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldUseTodoUpdateEditor(tc.hasUpdateFlags, tc.editFlag, tc.noEditFlag, tc.interactive)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestTodoListPriorityFilter(t *testing.T) {
	valid := todo.PriorityMedium
	priority, err := todoListPriorityFilter(valid, true)
	if err != nil {
		t.Fatalf("expected valid priority, got error: %v", err)
	}
	if priority == nil || *priority != valid {
		t.Fatalf("expected priority %d, got %v", valid, priority)
	}

	priority, err = todoListPriorityFilter(-1, false)
	if err != nil {
		t.Fatalf("expected no error when priority not set, got %v", err)
	}
	if priority != nil {
		t.Fatalf("expected nil priority when not set, got %v", priority)
	}

	priority, err = todoListPriorityFilter(-1, true)
	if err == nil || !errors.Is(err, todo.ErrInvalidPriority) {
		t.Fatalf("expected invalid priority error, got %v", err)
	}
}

func TestPrintTodoDetailIncludesDeleteMetadata(t *testing.T) {
	deletedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	item := todo.Todo{
		ID:           "abc12345",
		Title:        "Deprecated",
		Type:         todo.TypeTask,
		Status:       todo.StatusTombstone,
		Priority:     todo.PriorityLow,
		CreatedAt:    time.Date(2026, 1, 1, 1, 2, 3, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 1, 1, 2, 3, 4, 0, time.UTC),
		DeletedAt:    &deletedAt,
		DeleteReason: "no longer needed",
	}

	output := captureStdout(t, func() {
		printTodoDetail(item, func(id string) string { return id })
	})

	if !strings.Contains(output, "Deleted:  2026-01-02 03:04:05") {
		t.Fatalf("expected deleted timestamp in output, got: %q", output)
	}
	if !strings.Contains(output, "Delete Reason: no longer needed") {
		t.Fatalf("expected delete reason in output, got: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	defer func() {
		os.Stdout = old
		_ = r.Close()
	}()

	os.Stdout = w
	fn()
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	return buf.String()
}
