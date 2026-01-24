package opencode

import "time"

// AgeData computes the display age and whether timing data exists.
func AgeData(session OpencodeSession, now time.Time) (time.Duration, bool) {
	if session.CreatedAt.IsZero() {
		return 0, false
	}
	return now.Sub(session.CreatedAt), true
}

// Age computes the display age for an opencode session.
func Age(session OpencodeSession, now time.Time) time.Duration {
	aged, _ := AgeData(session, now)
	return aged
}

// DurationData computes the session duration and whether timing data exists.
func DurationData(session OpencodeSession, now time.Time) (time.Duration, bool) {
	if session.CreatedAt.IsZero() {
		return 0, false
	}
	if session.Status == OpencodeSessionActive {
		return now.Sub(session.CreatedAt), true
	}
	if session.UpdatedAt.IsZero() {
		return 0, false
	}
	return session.UpdatedAt.Sub(session.CreatedAt), true
}

// Duration computes the session duration.
func Duration(session OpencodeSession, now time.Time) time.Duration {
	duration, _ := DurationData(session, now)
	return duration
}
