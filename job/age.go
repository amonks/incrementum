package job

import "time"

// AgeData computes the display age and whether timing data exists.
func AgeData(item Job, now time.Time) (time.Duration, bool) {
	if item.CreatedAt.IsZero() {
		return 0, false
	}
	return now.Sub(item.CreatedAt), true
}

// Age computes the display age for a job.
func Age(item Job, now time.Time) time.Duration {
	ageValue, _ := AgeData(item, now)
	return ageValue
}

// DurationData computes the display duration and whether timing data exists.
func DurationData(item Job, now time.Time) (time.Duration, bool) {
	if item.CreatedAt.IsZero() {
		return 0, false
	}
	if item.Status == StatusActive {
		return now.Sub(item.CreatedAt), true
	}
	if item.UpdatedAt.IsZero() {
		return 0, false
	}
	return item.UpdatedAt.Sub(item.CreatedAt), true
}

// Duration computes the display duration for a job.
func Duration(item Job, now time.Time) time.Duration {
	duration, _ := DurationData(item, now)
	return duration
}
