package strings

import "testing"

func TestNormalizeWhitespace(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: " \n\t ",
			want:  "",
		},
		{
			name:  "single token",
			input: "topic",
			want:  "topic",
		},
		{
			name:  "collapses spaces",
			input: "one   two    three",
			want:  "one two three",
		},
		{
			name:  "collapses newlines",
			input: "one\n\n two\tthree",
			want:  "one two three",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeWhitespace(tc.input)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNormalizeLower(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already lower",
			input: "ready",
			want:  "ready",
		},
		{
			name:  "mixed case",
			input: "In_Progress",
			want:  "in_progress",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeLower(tc.input)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNormalizeLowerTrimSpace(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "trims and lowercases",
			input: "  DONE  ",
			want:  "done",
		},
		{
			name:  "inner spaces preserved",
			input: "  in progress  ",
			want:  "in progress",
		},
		{
			name:  "whitespace only",
			input: "  \t\n ",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeLowerTrimSpace(tc.input)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNormalizeNewlines(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "no carriage returns",
			input: "one\ntwo",
			want:  "one\ntwo",
		},
		{
			name:  "crlf",
			input: "one\r\ntwo",
			want:  "one\ntwo",
		},
		{
			name:  "cr only",
			input: "one\rtwo",
			want:  "one\ntwo",
		},
		{
			name:  "mixed",
			input: "one\r\ntwo\rthree",
			want:  "one\ntwo\nthree",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeNewlines(tc.input)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestTrimTrailingNewlines(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "no newline",
			input: "note",
			want:  "note",
		},
		{
			name:  "trailing newline",
			input: "note\n",
			want:  "note",
		},
		{
			name:  "trailing crlf",
			input: "note\r\n",
			want:  "note",
		},
		{
			name:  "multiple trailing",
			input: "note\n\r\n",
			want:  "note",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TrimTrailingNewlines(tc.input)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
