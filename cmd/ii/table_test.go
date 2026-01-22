package main

import (
	"strings"
	"testing"
)

func TestTruncateTableCellCountsRunes(t *testing.T) {
	value := strings.Repeat("a", tableCellMaxWidth-1) + "\u00e9"

	got := truncateTableCell(value)

	if got != value {
		t.Fatalf("expected value to remain untruncated, got %q", got)
	}
}

func TestTruncateTableCellNormalizesLineBreaks(t *testing.T) {
	value := "Hello\nWorld\r\nAgain\tTab"

	got := truncateTableCell(value)

	if got != "Hello World Again Tab" {
		t.Fatalf("expected line breaks to normalize, got %q", got)
	}
}
