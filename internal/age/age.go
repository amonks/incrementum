package age

import "time"

// AgeData computes display age and whether timing data exists.
func AgeData(startedAt time.Time, now time.Time) (time.Duration, bool) {
	if startedAt.IsZero() {
		return 0, false
	}
	age := now.Sub(startedAt)
	if age < 0 {
		age = 0
	}
	return age, true
}

// Age computes display age.
func Age(startedAt time.Time, now time.Time) time.Duration {
	ageValue, _ := AgeData(startedAt, now)
	return ageValue
}

// DurationData computes display duration and whether timing data exists.
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
