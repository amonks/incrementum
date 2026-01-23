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
