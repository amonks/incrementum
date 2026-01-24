package opencode

import (
	"os"
	"strings"
	"testing"
)

func TestResolveRunStdinUsesPromptWhenStdinNil(t *testing.T) {
	opts := RunOptions{Prompt: "Hello"}

	got := resolveRunStdin(opts)

	reader, ok := got.(*strings.Reader)
	if !ok {
		t.Fatalf("expected strings.Reader, got %T", got)
	}
	if reader.Len() != len(opts.Prompt) {
		t.Fatalf("expected reader length %d, got %d", len(opts.Prompt), reader.Len())
	}
}

func TestResolveRunStdinUsesOSStdinWhenPromptEmpty(t *testing.T) {
	opts := RunOptions{}

	got := resolveRunStdin(opts)

	if got != os.Stdin {
		t.Fatalf("expected os.Stdin, got %T", got)
	}
}

func TestResolveRunStdinPrefersProvidedStdin(t *testing.T) {
	stdin := strings.NewReader("input")
	opts := RunOptions{Prompt: "Hello", Stdin: stdin}

	got := resolveRunStdin(opts)

	if got != stdin {
		t.Fatalf("expected provided stdin, got %T", got)
	}
}
