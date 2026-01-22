package job

import (
	"time"

	"github.com/amonks/incrementum/internal/age"
)

// AgeData computes the display age and whether timing data exists.
func AgeData(item Job, now time.Time) (time.Duration, bool) {
	return age.DurationData(item.StartedAt, item.CompletedAt, 0, item.Status == StatusActive, now)
}

// Age computes the display age for a job.
func Age(item Job, now time.Time) time.Duration {
	ageValue, _ := AgeData(item, now)
	return ageValue
}
