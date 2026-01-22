package session

import (
	"testing"
	"time"
)

func TestAgeData(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)
	completed := start.Add(3 * time.Minute)

	cases := []struct {
		name    string
		session Session
		want    time.Duration
		ok      bool
	}{
		{
			name: "active uses now",
			session: Session{
				Status:    StatusActive,
				StartedAt: start,
			},
			want: 10 * time.Minute,
			ok:   true,
		},
		{
			name: "active zero duration",
			session: Session{
				Status:    StatusActive,
				StartedAt: now,
			},
			want: 0,
			ok:   true,
		},
		{
			name: "duration seconds preferred",
			session: Session{
				Status:          StatusCompleted,
				StartedAt:       start,
				CompletedAt:     now,
				DurationSeconds: 90,
			},
			want: 90 * time.Second,
			ok:   true,
		},
		{
			name: "completed uses timestamps",
			session: Session{
				Status:      StatusCompleted,
				StartedAt:   start,
				CompletedAt: completed,
			},
			want: 3 * time.Minute,
			ok:   true,
		},
		{
			name: "missing timing data",
			session: Session{
				Status: StatusActive,
			},
			want: 0,
			ok:   false,
		},
		{
			name: "completed missing timestamps",
			session: Session{
				Status:    StatusCompleted,
				StartedAt: start,
			},
			want: 0,
			ok:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := AgeData(tc.session, now)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("expected %s/%t, got %s/%t", tc.want, tc.ok, got, ok)
			}
		})
	}
}
