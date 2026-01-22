package main

import (
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func TestFormatOpencodeTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess1",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    "Run tests\nSecond line",
			StartedAt: start,
			UpdatedAt: start,
		},
		{
			ID:              "sess2",
			Status:          workspace.OpencodeSessionCompleted,
			Prompt:          "Build app",
			StartedAt:       start.Add(-time.Minute),
			UpdatedAt:       now,
			CompletedAt:     now,
			DurationSeconds: 90,
			ExitCode:        intPtr(0),
		},
	}

	plain := formatOpencodeTable(sessions, func(id string) string { return id }, now)
	ansi := formatOpencodeTable(sessions, func(id string) string {
		return "\x1b[1m\x1b[36m" + id + "\x1b[0m"
	}, now)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestOpencodeSessionAgeUsesDurationSeconds(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)

	session := workspace.OpencodeSession{
		Status:          workspace.OpencodeSessionCompleted,
		StartedAt:       start,
		CompletedAt:     now,
		DurationSeconds: 90,
	}

	age := opencodeSessionAge(session, now)
	if age != 90*time.Second {
		t.Fatalf("expected 90s duration, got %s", age)
	}
}

func TestFormatOpencodeTableIncludesSessionID(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-123",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    "Ship it",
			StartedAt: now.Add(-time.Minute),
			UpdatedAt: now.Add(-time.Minute),
		},
	}

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(id string) string { return id }, now))
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
	start := now.Add(-2 * time.Minute)

	sessions := []workspace.OpencodeSession{
		{
			ID:        "sess-001",
			Status:    workspace.OpencodeSessionActive,
			Prompt:    "Prompt",
			StartedAt: start,
			UpdatedAt: start,
		},
	}

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(id string) string { return id }, now))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		t.Fatalf("expected at least 4 columns, got: %q", lines[1])
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

	output := strings.TrimSpace(formatOpencodeTable(sessions, func(value string) string { return value }, now))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		t.Fatalf("expected at least 3 columns, got: %q", lines[1])
	}

	if fields[2] != "-" {
		t.Fatalf("expected missing age '-', got: %s", fields[2])
	}
}

func TestFilterOpencodeSessionsForListDefaultsToActive(t *testing.T) {
	sessions := []workspace.OpencodeSession{
		{ID: "active", Status: workspace.OpencodeSessionActive},
		{ID: "completed", Status: workspace.OpencodeSessionCompleted},
		{ID: "failed", Status: workspace.OpencodeSessionFailed},
		{ID: "killed", Status: workspace.OpencodeSessionKilled},
	}

	filtered := filterOpencodeSessionsForList(sessions, false)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(filtered))
	}
	if filtered[0].ID != "active" {
		t.Fatalf("expected active session, got %q", filtered[0].ID)
	}
}

func TestFilterOpencodeSessionsForListWithAll(t *testing.T) {
	sessions := []workspace.OpencodeSession{
		{ID: "active", Status: workspace.OpencodeSessionActive},
		{ID: "completed", Status: workspace.OpencodeSessionCompleted},
	}

	filtered := filterOpencodeSessionsForList(sessions, true)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(filtered))
	}
}
