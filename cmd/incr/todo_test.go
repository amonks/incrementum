package main

import (
	"bytes"
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
