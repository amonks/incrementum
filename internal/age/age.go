package age

import "time"

// DurationData computes display age and whether timing data exists.
func DurationData(startedAt time.Time, completedAt time.Time, durationSeconds int, active bool, now time.Time) (time.Duration, bool) {
	if active {
		if startedAt.IsZero() {
			return 0, false
		}
		return now.Sub(startedAt), true
	}

	if durationSeconds > 0 {
		return time.Duration(durationSeconds) * time.Second, true
	}

	if !completedAt.IsZero() && !startedAt.IsZero() {
		return completedAt.Sub(startedAt), true
	}

	return 0, false
}
