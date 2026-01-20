package main

import (
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

	plain := formatSessionTable(sessions, func(id string, prefix int) string { return id }, now)
	ansi := formatSessionTable(sessions, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now)

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

func intPtr(value int) *int {
	return &value
}
