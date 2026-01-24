package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/workspace"
	"github.com/charmbracelet/lipgloss"
)

func TestFormatOpencodeTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess1",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    "Run tests\nSecond line",
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:              "sess2",
			Status:          workspace.OpencodeSessionCompleted,
			Prompt:          "Build app",
			CreatedAt:       createdAt.Add(-time.Minute),
			StartedAt:       createdAt.Add(-time.Minute),
			UpdatedAt:       now,
			CompletedAt:     now,
			DurationSeconds: 90,
			ExitCode:        intPtr(0),
		},
	}

	plain := formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil)
	ansi := formatOpencodeTable(sessions, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now, nil)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatOpencodeTableTruncatesPromptToViewport(t *testing.T) {
	restore := ui.OverrideTableViewportWidth(func() int {
		return 60
	})
	t.Cleanup(restore)

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-1",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    strings.Repeat("a", 120),
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	output := strings.TrimSuffix(formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil), "\n")
	lines := strings.Split(output, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and row only, got %d lines in %q", len(lines), output)
	}

	if !strings.Contains(lines[1], "...") {
		t.Fatalf("expected prompt to truncate with ellipsis, got %q", lines[1])
	}

	if width := ui.TableCellWidth(lines[1]); width != 60 {
		t.Fatalf("expected row width 60, got %d in %q", width, lines[1])
	}
}

func TestFormatOpencodeTableTruncatesWidePromptToViewport(t *testing.T) {
	restore := ui.OverrideTableViewportWidth(func() int {
		return 60
	})
	t.Cleanup(restore)

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-wide",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    strings.Repeat("\u754c", 80),
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	output := strings.TrimSuffix(formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil), "\n")
	lines := strings.Split(output, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and row only, got %d lines in %q", len(lines), output)
	}

	if !strings.Contains(lines[1], "...") {
		t.Fatalf("expected prompt to truncate with ellipsis, got %q", lines[1])
	}

	if width := lipgloss.Width(lines[1]); width != 60 {
		t.Fatalf("expected row width 60, got %d in %q", width, lines[1])
	}
}

func TestFormatOpencodeTableTruncatesPromptHeaderToViewport(t *testing.T) {
	restore := ui.OverrideTableViewportWidth(func() int {
		return 40
	})
	t.Cleanup(restore)

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-1",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    strings.Repeat("b", 40),
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	output := strings.TrimSuffix(formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil), "\n")
	lines := strings.Split(output, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and row only, got %d lines in %q", len(lines), output)
	}

	if strings.Contains(lines[0], "PROMPT") {
		t.Fatalf("expected prompt header to truncate, got %q", lines[0])
	}

	if width := ui.TableCellWidth(lines[0]); width != 40 {
		t.Fatalf("expected header width 40, got %d in %q", width, lines[0])
	}
	if width := ui.TableCellWidth(lines[1]); width != 40 {
		t.Fatalf("expected row width 40, got %d in %q", width, lines[1])
	}
}

func TestFormatOpencodeTableIncludesSessionID(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-123",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    "Ship it",
			CreatedAt: now.Add(-time.Minute),
			StartedAt: now.Add(-time.Minute),
			UpdatedAt: now.Add(-time.Minute),
		},
	}

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	header := lines[0]
	sessionIndex := strings.Index(header, "SESSION")
	statusIndex := strings.Index(header, "STATUS")
	if sessionIndex == -1 || statusIndex == -1 || sessionIndex > statusIndex {
		t.Fatalf("expected SESSION column before STATUS in header, got: %q", header)
	}

	row := lines[1]
	if !strings.Contains(row, "sess-123") {
		t.Fatalf("expected session id in row, got: %q", row)
	}
}

func TestFormatOpencodeTableUsesCompactAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-001",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    "Prompt",
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		t.Fatalf("expected at least 5 columns, got: %q", lines[1])
	}

	if fields[2] != "2m" {
		t.Fatalf("expected compact age 2m, got: %s", fields[2])
	}
}

func TestFormatOpencodeTableShowsMissingAgeAsDash(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []workspace.OpencodeSession{
		{
			ID:     "sess-1",
			Status: workspace.OpencodeSessionActive,
			Prompt: "Do the thing",
		},
	}

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(value string, prefix int) string { return value }, now, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		t.Fatalf("expected at least 4 columns, got: %q", lines[1])
	}

	if fields[2] != "-" {
		t.Fatalf("expected age -, got: %s", fields[2])
	}
}

func TestFormatOpencodeTableShowsAgeForCompletedSession(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-complete",
			Status:    workspace.OpencodeSessionCompleted,
			Prompt:    "Done",
			CreatedAt: now.Add(-5 * time.Minute),
			StartedAt: now.Add(-5 * time.Minute),
		},
	}

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		t.Fatalf("expected at least 4 columns, got: %q", lines[1])
	}

	if fields[2] != "5m" {
		t.Fatalf("expected age 5m, got: %s", fields[2])
	}
}

func TestFormatOpencodeTableShowsDuration(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-10 * time.Minute)
	updated := now.Add(-7 * time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-duration",
			Status:    workspace.OpencodeSessionCompleted,
			Prompt:    "Do it",
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: updated,
		},
	}

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(id string, prefix int) string { return id }, now, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		t.Fatalf("expected at least 4 columns, got: %q", lines[1])
	}

	if fields[3] != "3m" {
		t.Fatalf("expected duration 3m, got: %s", fields[3])
	}
}

func TestOpencodePromptLineTreatsWhitespaceAsMissing(t *testing.T) {
	prompt := "   \nsecond line"
	if got := opencodePromptLine(prompt); got != "-" {
		t.Fatalf("expected whitespace-only prompt to return '-', got %q", got)
	}
}

func TestFormatOpencodeTableUsesSessionPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-5 * time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "abc123",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    "One",
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:          "abd999",
			Status:      workspace.OpencodeSessionCompleted,
			Prompt:      "Two",
			CreatedAt:   createdAt,
			StartedAt:   createdAt,
			UpdatedAt:   now,
			CompletedAt: now,
		},
	}

	output := formatOpencodeTable(sessions, func(id string, prefix int) string {
		return id + ":" + strconv.Itoa(prefix)
	}, now, nil)

	if !strings.Contains(output, "abc123:3") {
		t.Fatalf("expected session prefix length 3, got: %q", output)
	}
	if !strings.Contains(output, "abd999:3") {
		t.Fatalf("expected session prefix length 3, got: %q", output)
	}
}

func intPtr(value int) *int {
	return &value
}
