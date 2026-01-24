package main

import (
	"testing"
	"time"

	"github.com/amonks/incrementum/opencode"
)

func TestFormatOpencodeAgeUsesCreatedAtForActive(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-10 * time.Minute)

	session := opencode.OpencodeSession{
		Status:    opencode.OpencodeSessionActive,
		CreatedAt: createdAt,
	}

	if got := formatOpencodeAge(session, now); got != "10m" {
		t.Fatalf("expected 10m, got %q", got)
	}
}

func TestFormatOpencodeAgeUsesCreatedAtForCompleted(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-10 * time.Minute)

	session := opencode.OpencodeSession{
		Status:      opencode.OpencodeSessionCompleted,
		CreatedAt:   createdAt,
		CompletedAt: now,
	}

	if got := formatOpencodeAge(session, now); got != "10m" {
		t.Fatalf("expected 10m, got %q", got)
	}
}

func TestFormatOpencodeAgeHandlesMissingTimingData(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	session := opencode.OpencodeSession{Status: opencode.OpencodeSessionActive}

	if got := formatOpencodeAge(session, now); got != "-" {
		t.Fatalf("expected -, got %q", got)
	}
}

func TestFormatOpencodeDurationUsesNowForActive(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-7 * time.Minute)

	session := opencode.OpencodeSession{
		Status:    opencode.OpencodeSessionActive,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	if got := formatOpencodeDuration(session, now); got != "7m" {
		t.Fatalf("expected 7m, got %q", got)
	}
}

func TestFormatOpencodeDurationUsesUpdatedAtForCompleted(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	createdAt := now.Add(-20 * time.Minute)
	updated := now.Add(-15 * time.Minute)

	session := opencode.OpencodeSession{
		Status:    opencode.OpencodeSessionCompleted,
		CreatedAt: createdAt,
		UpdatedAt: updated,
	}

	if got := formatOpencodeDuration(session, now); got != "5m" {
		t.Fatalf("expected 5m, got %q", got)
	}
}

func TestFormatOpencodeDurationHandlesMissingTimingData(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	session := opencode.OpencodeSession{Status: opencode.OpencodeSessionCompleted}

	if got := formatOpencodeDuration(session, now); got != "-" {
		t.Fatalf("expected -, got %q", got)
	}
}
