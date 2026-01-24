package opencode

import (
	"time"

	internalage "github.com/amonks/incrementum/internal/age"
)

// AgeData computes the display age and whether timing data exists.
func AgeData(session OpencodeSession, now time.Time) (time.Duration, bool) {
	return internalage.AgeData(session.CreatedAt, now)
}

// Age computes the display age for an opencode session.
func Age(session OpencodeSession, now time.Time) time.Duration {
	return internalage.Age(session.CreatedAt, now)
}

// DurationData computes the session duration and whether timing data exists.
func DurationData(session OpencodeSession, now time.Time) (time.Duration, bool) {
	return internalage.DurationData(session.CreatedAt, session.UpdatedAt, 0, session.Status == OpencodeSessionActive, now)
}

// Duration computes the session duration.
func Duration(session OpencodeSession, now time.Time) time.Duration {
	duration, _ := DurationData(session, now)
	return duration
}
