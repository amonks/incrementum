package job

import (
	"testing"
	"time"
)

func TestAgeDataActiveJobUsesStartTime(t *testing.T) {
	startedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	now := startedAt.Add(2 * time.Minute)

	item := Job{
		Status:    StatusActive,
		StartedAt: startedAt,
	}

	age, ok := AgeData(item, now)
	if !ok {
		t.Fatalf("expected age data")
	}
	if age != 2*time.Minute {
		t.Fatalf("expected age 2m, got %s", age)
	}
}

func TestAgeDataCompletedJobUsesCompletedAt(t *testing.T) {
	startedAt := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(5 * time.Minute)

	item := Job{
		Status:      StatusCompleted,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}

	age, ok := AgeData(item, completedAt.Add(time.Minute))
	if !ok {
		t.Fatalf("expected age data")
	}
	if age != 5*time.Minute {
		t.Fatalf("expected age 5m, got %s", age)
	}
}
