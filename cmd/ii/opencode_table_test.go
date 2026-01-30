package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/opencode"
)

func TestFormatOpencodeTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)

	sessions := []opencode.OpencodeSession{
		{
			ID:        "sess1",
			Status:    opencode.OpencodeSessionActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:              "sess2",
			Status:          opencode.OpencodeSessionCompleted,
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

func trimmedOpencodeTable(sessions []opencode.OpencodeSession, highlight func(string, int) string, now time.Time) string {
	return internalstrings.TrimSpace(formatOpencodeTable(sessions, highlight, now, nil))
}

func TestFormatOpencodeTableIncludesSessionID(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []opencode.OpencodeSession{
		{
			ID:        "sess-123",
			Status:    opencode.OpencodeSessionActive,
			CreatedAt: now.Add(-time.Minute),
			StartedAt: now.Add(-time.Minute),
			UpdatedAt: now.Add(-time.Minute),
		},
	}

	output := trimmedOpencodeTable(sessions, func(id string, prefix int) string { return id }, now)
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

	sessions := []opencode.OpencodeSession{
		{
			ID:        "sess-001",
			Status:    opencode.OpencodeSessionActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	output := trimmedOpencodeTable(sessions, func(id string, prefix int) string { return id }, now)
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

	sessions := []opencode.OpencodeSession{
		{
			ID:     "sess-1",
			Status: opencode.OpencodeSessionActive,
		},
	}

	output := trimmedOpencodeTable(sessions, func(value string, prefix int) string { return value }, now)
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

	sessions := []opencode.OpencodeSession{
		{
			ID:        "sess-complete",
			Status:    opencode.OpencodeSessionCompleted,
			CreatedAt: now.Add(-5 * time.Minute),
			StartedAt: now.Add(-5 * time.Minute),
		},
	}

	output := trimmedOpencodeTable(sessions, func(id string, prefix int) string { return id }, now)
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

	sessions := []opencode.OpencodeSession{
		{
			ID:        "sess-duration",
			Status:    opencode.OpencodeSessionCompleted,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: updated,
		},
	}

	output := trimmedOpencodeTable(sessions, func(id string, prefix int) string { return id }, now)
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

func TestFormatOpencodeTableUsesSessionPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-5 * time.Minute)

	sessions := []opencode.OpencodeSession{
		{
			ID:        "abc123",
			Status:    opencode.OpencodeSessionActive,
			CreatedAt: createdAt,
			StartedAt: createdAt,
			UpdatedAt: createdAt,
		},
		{
			ID:          "abd999",
			Status:      opencode.OpencodeSessionCompleted,
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

func TestFormatOpencodeTableShowsExitCode(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-5 * time.Minute)

	sessions := []opencode.OpencodeSession{
		{
			ID:          "sess-exit",
			Status:      opencode.OpencodeSessionFailed,
			CreatedAt:   createdAt,
			StartedAt:   createdAt,
			UpdatedAt:   now,
			CompletedAt: now,
			ExitCode:    intPtr(1),
		},
	}

	output := trimmedOpencodeTable(sessions, func(id string, prefix int) string { return id }, now)
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	// Exit code should be the last column
	fields := strings.Fields(lines[1])
	lastField := fields[len(fields)-1]
	if lastField != "1" {
		t.Fatalf("expected exit code 1, got: %s", lastField)
	}
}

func intPtr(value int) *int {
	return &value
}
