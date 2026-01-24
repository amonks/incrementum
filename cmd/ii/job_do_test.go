package main

import (
	"strings"
	"testing"
)

func TestReflowJobTextPreservesParagraphs(t *testing.T) {
	input := "First paragraph line one.\nSecond line stays with paragraph.\n\nSecond paragraph follows."
	output := reflowJobText(input, 80)

	if output == "-" {
		t.Fatalf("expected non-empty output, got %q", output)
	}
	if output == input {
		t.Fatalf("expected reflowing to normalize whitespace, got %q", output)
	}
	if !strings.Contains(output, "\n\n") {
		t.Fatalf("expected paragraph break preserved, got %q", output)
	}
	if !strings.Contains(output, "First paragraph line one. Second line stays with paragraph.") {
		t.Fatalf("expected paragraph to reflow, got %q", output)
	}
}
