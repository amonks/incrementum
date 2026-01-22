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
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
		{
			ID:          "job-2",
			TodoID:      "abd99999",
			Stage:       jobpkg.StageReviewing,
			Status:      jobpkg.StatusCompleted,
			StartedAt:   startedAt.Add(-time.Minute),
			UpdatedAt:   now,
			CompletedAt: now,
		},
	}

	plain := formatJobTable(jobs, func(id string, prefix int) string { return id }, now, nil, nil)
	ansi := formatJobTable(jobs, func(id string, prefix int) string {
		if prefix <= 0 || prefix > len(id) {
			return id
		}
		return "\x1b[1m\x1b[36m" + id[:prefix] + "\x1b[0m" + id[prefix:]
	}, now, nil, nil)

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
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
		{
			ID:        "job-beta",
			TodoID:    "abd99999",
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
	}, now, nil, jobPrefixes)

	if !strings.Contains(output, "job-alpha:2") {
		t.Fatalf("expected job prefix length 2, got: %q", output)
	}
	if !strings.Contains(output, "job-beta:3") {
		t.Fatalf("expected job prefix length 3, got: %q", output)
	}
}

func TestFormatJobTableUsesCompactAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-2 * time.Minute)

	jobs := []jobpkg.Job{
		{
			ID:        "job-age",
			TodoID:    "abc12345",
			Stage:     jobpkg.StageImplementing,
			Status:    jobpkg.StatusActive,
			StartedAt: startedAt,
			UpdatedAt: startedAt,
		},
	}

	output := strings.TrimSpace(formatJobTable(jobs, func(id string, prefix int) string { return id }, now, nil, nil))
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
