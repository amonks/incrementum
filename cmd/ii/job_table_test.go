package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	jobpkg "github.com/amonks/incrementum/job"
)

func TestFormatJobTablePreservesAlignmentWithANSI(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-2 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-1",
			TodoID:    "abc12345",
			SessionID: "sess-1",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
		{
			ID:          "job-2",
			TodoID:      "abd99999",
			SessionID:   "sess-2",
			Stage:       jobpkg.StageReviewing,
			Status:      jobpkg.StatusCompleted,
			StartedAt:   startedAt.Add(-time.Minute),
			UpdatedAt:   now,
			CompletedAt: now,
		},
	}

	plain := formatJobTable(jobs, func(id string, prefix int) string { return id }, now, nil, nil, nil)
	ansi := formatJobTable(jobs, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now, nil, nil, nil)

	if stripANSICodes(ansi) != plain {
		t.Fatalf("expected ANSI output to align with plain output\nplain:\n%s\nansi:\n%s", plain, ansi)
	}
}

func TestFormatJobTableUsesProvidedJobPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-5 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-alpha",
			TodoID:    "abc12345",
			SessionID: "sess-alpha",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
		{
			ID:        "job-beta",
			TodoID:    "abd99999",
			SessionID: "sess-beta",
			Stage:     jobpkg.StageTesting,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
	}

	jobPrefixes := map[string]int{
		"job-alpha": 2,
		"job-beta":  3,
	}

	output := formatJobTable(jobs, func(id string, prefix int) string {
		return id + ":" + strconv.Itoa(prefix)
	}, now, nil, jobPrefixes, nil)

	if !strings.Contains(output, "job-alpha:2") {
		t.Fatalf("expected job prefix length 2, got: %q", output)
	}
	if !strings.Contains(output, "job-beta:3") {
		t.Fatalf("expected job prefix length 3, got: %q", output)
	}
}

func TestFormatJobTableUsesProvidedSessionPrefixLengths(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-5 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-alpha",
			TodoID:    "abc12345",
			SessionID: "session-alpha",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
		{
			ID:        "job-beta",
			TodoID:    "abd99999",
			SessionID: "session-beta",
			Stage:     jobpkg.StageTesting,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
	}

	sessionPrefixes := map[string]int{
		"session-alpha": 2,
		"session-beta":  3,
	}

	output := formatJobTable(jobs, func(id string, prefix int) string {
		return id + ":" + strconv.Itoa(prefix)
	}, now, nil, nil, sessionPrefixes)

	if !strings.Contains(output, "session-alpha:2") {
		t.Fatalf("expected session prefix length 2, got: %q", output)
	}
	if !strings.Contains(output, "session-beta:3") {
		t.Fatalf("expected session prefix length 3, got: %q", output)
	}
}

func TestFormatJobTableUsesCompactAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-2 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-age",
			TodoID:    "abc12345",
			SessionID: "sess-age",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
	}

	output := strings.TrimSpace(formatJobTable(jobs, func(id string, prefix int) string { return id }, now, nil, nil, nil))
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got: %q", output)
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		t.Fatalf("expected at least 6 columns, got: %q", lines[1])
	}

	if fields[5] != "2m" {
		t.Fatalf("expected compact age 2m, got: %s", fields[5])
	}
}
