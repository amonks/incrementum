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
