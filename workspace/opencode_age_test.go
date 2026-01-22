package workspace

import (
	"testing"
	"time"
)

func TestOpencodeSessionAge(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-10 * time.Minute)
	completed := start.Add(3 * time.Minute)

	cases := []struct {
		name    string
		session OpencodeSession
		want    time.Duration
	}{
		{
			name: "active uses now",
			session: OpencodeSession{
				Status:    OpencodeSessionActive,
				StartedAt: start,
			},
			want: 10 * time.Minute,
		},
		{
			name: "duration seconds preferred",
			session: OpencodeSession{
				Status:          OpencodeSessionCompleted,
				StartedAt:       start,
				CompletedAt:     now,
				DurationSeconds: 90,
			},
			want: 90 * time.Second,
		},
		{
			name: "completed uses timestamps",
			session: OpencodeSession{
				Status:      OpencodeSessionCompleted,
				StartedAt:   start,
				CompletedAt: completed,
			},
			want: 3 * time.Minute,
		},
		{
			name: "missing timing data",
			session: OpencodeSession{
				Status: OpencodeSessionActive,
			},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := OpencodeSessionAge(tc.session, now)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}
