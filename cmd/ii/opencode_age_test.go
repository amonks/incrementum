package main

import (
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func TestFormatOpencodeAgeUsesStartedAtForActive(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)

	session := workspace.OpencodeSession{
		Status:    workspace.OpencodeSessionActive,
		StartedAt: start,
	}

	if got := formatOpencodeAge(session, now); got != "10m" {
		t.Fatalf("expected 10m, got %q", got)
	}
}

func TestFormatOpencodeAgeUsesDurationSeconds(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)

	session := workspace.OpencodeSession{
		Status:          workspace.OpencodeSessionCompleted,
		StartedAt:       start,
		CompletedAt:     now,
		DurationSeconds: 90,
	}

	if got := formatOpencodeAge(session, now); got != "1m" {
		t.Fatalf("expected 1m, got %q", got)
	}
}

func TestFormatOpencodeAgeHandlesMissingTimingData(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	session := workspace.OpencodeSession{Status: workspace.OpencodeSessionActive}

	if got := formatOpencodeAge(session, now); got != "-" {
		t.Fatalf("expected -, got %q", got)
	}
}
