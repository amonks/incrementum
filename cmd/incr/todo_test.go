package main

import (
	"bytes"
	"fmt"
	"testing"
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

func TestTodoLogHighlighterUsesUniquePrefixes(t *testing.T) {
	highlight := todoLogHighlighter([]string{"abc123", "abd456"}, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	})

	if got := highlight("abc123"); got != "abc123:3" {
		t.Fatalf("expected abc123 to use prefix 3, got %q", got)
	}
	if got := highlight("abd456"); got != "abd456:3" {
		t.Fatalf("expected abd456 to use prefix 3, got %q", got)
	}
}

func TestTodoLogHighlighterHandlesSingleID(t *testing.T) {
	highlight := todoLogHighlighter([]string{"abc123"}, func(id string, prefix int) string {
		return fmt.Sprintf("%s:%d", id, prefix)
	})

	if got := highlight("abc123"); got != "abc123:1" {
		t.Fatalf("expected abc123 to use prefix 1, got %q", got)
	}
}
