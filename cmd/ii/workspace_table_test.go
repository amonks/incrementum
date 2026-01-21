package main

import (
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func TestFormatWorkspaceTablePreservesAlignmentWithANSI(t *testing.T) {
	items := []workspace.Info{
		{
			Name:         "ws-001",
			Path:         "/tmp/ws-001",
			Purpose:      "feature work",
			Status:       workspace.StatusAvailable,
			TTLRemaining: 0,
		},
		{
			Name:         "ws-010",
			Path:         "/tmp/ws-010",
			Purpose:      "bugfix",
			Status:       workspace.StatusAcquired,
			TTLRemaining: 2*time.Minute + 5*time.Second,
		},
	}

	plain := formatWorkspaceTable(items, func(value string) string { return value })
	ansi := formatWorkspaceTable(items, func(value string) string {
		return "\x1b[1m\x1b[36m" + value + "\x1b[0m"
	})

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatWorkspaceTableTruncatesLongPaths(t *testing.T) {
	longPath := "/tmp/" + strings.Repeat("a", 60)
	items := []workspace.Info{
		{
			Name:         "ws-002",
			Path:         longPath,
			Purpose:      "refactor",
			Status:       workspace.StatusAvailable,
			TTLRemaining: 0,
		},
	}

	output := formatWorkspaceTable(items, nil)
	expected := longPath[:47] + "..."
	if !strings.Contains(output, expected) {
		t.Fatalf("expected truncated path %q in output: %s", expected, output)
	}
	if strings.Contains(output, longPath) {
		t.Fatalf("expected long path to be truncated, got: %s", output)
	}
}
