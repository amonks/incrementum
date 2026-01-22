package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	sessionpkg "github.com/amonks/incrementum/session"
)

func TestFormatSessionTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Minute)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess1",
			TodoID:        "abc12345",
			WorkspaceName: "ws-001",
			Status:        sessionpkg.StatusActive,
			Topic:         "Run tests",
			StartedAt:     start,
			UpdatedAt:     start,
		},
		{
			ID:              "sess2",
			TodoID:          "abd99999",
			WorkspaceName:   "ws-002",
			Status:          sessionpkg.StatusCompleted,
			Topic:           "Build app",
			StartedAt:       start.Add(-time.Minute),
			UpdatedAt:       now,
			CompletedAt:     now,
			DurationSeconds: 90,
			ExitCode:        intPtr(0),
		},
	}

	plain := formatSessionTable(sessions, func(id string, prefix int) string { return id }, now, nil, nil)
	ansi := formatSessionTable(sessions, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now, nil, nil)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestSessionAgeUsesDurationSeconds(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)

	session := sessionpkg.Session{
		Status:          sessionpkg.StatusCompleted,
		StartedAt:       start,
		CompletedAt:     now,
		DurationSeconds: 90,
	}

	age := sessionpkg.Age(session, now)
	if age != 90*time.Second {
		t.Fatalf("expected 90s duration, got %s", age)
	}
}

func TestFormatSessionTableUsesTodoPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-5 * time.Minute)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess-1",
			TodoID:        "abc12345",
			WorkspaceName: "ws-001",
			Status:        sessionpkg.StatusActive,
			Topic:         "One",
			StartedAt:     start,
			UpdatedAt:     start,
		},
		{
			ID:            "sess-2",
			TodoID:        "abd99999",
			WorkspaceName: "ws-002",
			Status:        sessionpkg.StatusCompleted,
			Topic:         "Two",
			StartedAt:     start,
			UpdatedAt:     start,
			CompletedAt:   now,
		},
	}

	todoPrefixLengths := map[string]int{
		"abc12345": 5,
		"abd99999": 4,
	}

	output := formatSessionTable(sessions, func(id string, prefix int) string {
		return id + ":" + strconv.Itoa(prefix)
	}, now, todoPrefixLengths, nil)

	if !strings.Contains(output, "abc12345:5") {
		t.Fatalf("expected todo prefix length 5, got: %q", output)
	}
	if !strings.Contains(output, "abd99999:4") {
		t.Fatalf("expected todo prefix length 4, got: %q", output)
	}
}

func TestFormatSessionTableFallsBackForMissingTodoPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-5 * time.Minute)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess-1",
			TodoID:        "abc12345",
			WorkspaceName: "ws-001",
			Status:        sessionpkg.StatusActive,
			Topic:         "One",
			StartedAt:     start,
			UpdatedAt:     start,
		},
		{
			ID:            "sess-2",
			TodoID:        "abd99999",
			WorkspaceName: "ws-002",
			Status:        sessionpkg.StatusCompleted,
			Topic:         "Two",
			StartedAt:     start,
			UpdatedAt:     start,
			CompletedAt:   now,
		},
	}

	todoPrefixLengths := map[string]int{
		"abc12345": 2,
	}

	output := formatSessionTable(sessions, func(id string, prefix int) string {
		return id + ":" + strconv.Itoa(prefix)
	}, now, todoPrefixLengths, nil)

	if !strings.Contains(output, "abc12345:3") {
		t.Fatalf("expected fallback prefix length 3, got: %q", output)
	}
	if !strings.Contains(output, "abd99999:3") {
		t.Fatalf("expected fallback prefix length 3, got: %q", output)
	}
}

func TestFormatSessionTableIncludesSessionID(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess-123",
			TodoID:        "abc12345",
			WorkspaceName: "ws-001",
			Status:        sessionpkg.StatusActive,
			Topic:         "Ship",
			StartedAt:     now.Add(-time.Minute),
			UpdatedAt:     now.Add(-time.Minute),
		},
	}

	output := strings.TrimSpace(formatSessionTable(sessions, func(id string, prefix int) string { return id }, now, nil, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	header := lines[0]
	sessionIndex := strings.Index(header, "SESSION")
	todoIndex := strings.Index(header, "TODO")
	if sessionIndex == -1 || todoIndex == -1 || sessionIndex > todoIndex {
		t.Fatalf("expected SESSION column before TODO in header, got: %q", header)
	}

	row := lines[1]
	if !strings.Contains(row, "sess-123") {
		t.Fatalf("expected session id in row, got: %q", row)
	}
}

func TestFormatSessionTableUsesCompactAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Minute)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess-321",
			TodoID:        "abc12345",
			WorkspaceName: "ws-001",
			Status:        sessionpkg.StatusActive,
			Topic:         "Topic",
			StartedAt:     start,
			UpdatedAt:     start,
		},
	}

	output := strings.TrimSpace(formatSessionTable(sessions, func(id string, prefix int) string { return id }, now, nil, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		t.Fatalf("expected at least 5 columns, got: %q", lines[1])
	}

	if fields[4] != "2m" {
		t.Fatalf("expected compact age 2m, got: %s", fields[4])
	}
}

func TestFormatSessionTableShowsMissingAgeAsDash(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess-missing",
			TodoID:        "abc12345",
			WorkspaceName: "ws-001",
			Status:        sessionpkg.StatusActive,
			Topic:         "No start",
		},
	}

	output := strings.TrimSpace(formatSessionTable(sessions, func(id string, prefix int) string { return id }, now, nil, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		t.Fatalf("expected at least 5 columns, got: %q", lines[1])
	}

	if fields[4] != "-" {
		t.Fatalf("expected missing age '-', got: %s", fields[4])
	}
}

func TestFormatSessionTableUsesProvidedSessionPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-5 * time.Minute)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess-alpha",
			TodoID:        "abc12345",
			WorkspaceName: "ws-001",
			Status:        sessionpkg.StatusActive,
			Topic:         "One",
			StartedAt:     start,
			UpdatedAt:     start,
		},
		{
			ID:            "sess-beta",
			TodoID:        "abd99999",
			WorkspaceName: "ws-002",
			Status:        sessionpkg.StatusCompleted,
			Topic:         "Two",
			StartedAt:     start,
			UpdatedAt:     start,
			CompletedAt:   now,
		},
	}

	providedPrefixes := map[string]int{
		"sess-alpha": 2,
		"sess-beta":  3,
	}

	output := formatSessionTable(sessions, func(id string, prefix int) string {
		return id + ":" + strconv.Itoa(prefix)
	}, now, nil, providedPrefixes)

	if !strings.Contains(output, "sess-alpha:2") {
		t.Fatalf("expected session prefix length 2, got: %q", output)
	}
	if !strings.Contains(output, "sess-beta:3") {
		t.Fatalf("expected session prefix length 3, got: %q", output)
	}
}

func TestFormatSessionTableShowsMissingAgeForCompletedSession(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []sessionpkg.Session{
		{
			ID:            "sess-complete",
			TodoID:        "abc12345",
			WorkspaceName: "ws-010",
			Status:        sessionpkg.StatusCompleted,
			Topic:         "Done",
			StartedAt:     now.Add(-5 * time.Minute),
		},
	}

	output := strings.TrimSpace(formatSessionTable(sessions, func(id string, prefix int) string { return id }, now, nil, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		t.Fatalf("expected at least 5 columns, got: %q", lines[1])
	}

	if fields[4] != "-" {
		t.Fatalf("expected missing age '-', got: %s", fields[4])
	}
}

func intPtr(value int) *int {
	return &value
}
