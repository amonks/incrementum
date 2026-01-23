package ui

import (
	"strings"
	"testing"
)

func TestTruncateTableCellCountsRunes(t *testing.T) {
	value := strings.Repeat("a", tableCellMaxWidth-1) + "\u00e9"

	got := TruncateTableCell(value)

	if got != value {
		t.Fatalf("expected value to remain untruncated, got %q", got)
	}
}

func TestTruncateTableCellNormalizesLineBreaks(t *testing.T) {
	value := "Hello\nWorld\r\nAgain\tTab"

	got := TruncateTableCell(value)

	if got != "Hello World Again Tab" {
		t.Fatalf("expected line breaks to normalize, got %q", got)
	}
}

func TestTruncateTableCellIgnoresANSICodes(t *testing.T) {
	value := "\x1b[1m\x1b[36m" + strings.Repeat("a", tableCellMaxWidth) + "\x1b[0m"

	got := TruncateTableCell(value)

	if got != value {
		t.Fatalf("expected value to remain untruncated, got %q", got)
	}
}

func TestFormatTableNormalizesLineBreaks(t *testing.T) {
	headers := []string{"COL"}
	rows := [][]string{{"Hello\nWorld\r\nAgain\tTab"}}

	got := FormatTable(headers, rows)

	expected := "COL\nHello World Again Tab\n"
	if got != expected {
		t.Fatalf("expected normalized table output, got %q", got)
	}
}
