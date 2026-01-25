package swarmtui

import (
	"testing"

	"github.com/amonks/incrementum/todo"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty", input: "", want: todo.PriorityMedium},
		{name: "numeric", input: "1", want: todo.PriorityHigh},
		{name: "name", input: "low", want: todo.PriorityLow},
		{name: "invalid", input: "bad", wantErr: true},
		{name: "range", input: "9", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePriority(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	status, err := parseStatus("in progress")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != todo.StatusInProgress {
		t.Fatalf("expected %q, got %q", todo.StatusInProgress, status)
	}

	if _, err := parseStatus("not-a-status"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseTodoType(t *testing.T) {
	value, err := parseTodoType("Bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != todo.TypeBug {
		t.Fatalf("expected %q, got %q", todo.TypeBug, value)
	}

	if _, err := parseTodoType("nope"); err == nil {
		t.Fatalf("expected error")
	}
}
