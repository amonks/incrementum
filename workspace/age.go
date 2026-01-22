package workspace

import (
	"time"

	"github.com/amonks/incrementum/internal/age"
)

// AgeData computes the display age and whether timing data exists.
func AgeData(session OpencodeSession, now time.Time) (time.Duration, bool) {
	return age.DurationData(session.StartedAt, session.CompletedAt, session.DurationSeconds, session.Status == OpencodeSessionActive, now)
}

// Age computes the display age for an opencode session.
func Age(session OpencodeSession, now time.Time) time.Duration {
	aged, _ := AgeData(session, now)
	return aged
}
