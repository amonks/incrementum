package ui

import (
	"fmt"
	"time"
)

// FormatTimeAgo returns a compact age string like "2m ago".
func FormatTimeAgo(then time.Time, now time.Time) string {
	duration, ok := timeAgeDuration(then, now)
	if !ok {
		return "-"
	}

	return FormatDurationShort(duration) + " ago"
}

// FormatTimeAgeShort returns a compact age string like "2m".
func FormatTimeAgeShort(then time.Time, now time.Time) string {
	duration, ok := timeAgeDuration(then, now)
	if !ok {
		return "-"
	}

	return FormatDurationShort(duration)
}

// FormatDurationShort formats a duration using short units (s/m/h/d).
func FormatDurationShort(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	duration = duration.Truncate(time.Second)
	seconds := int64(duration.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}

	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}

	days := hours / 24
	return fmt.Sprintf("%dd", days)
}

func timeAgeDuration(then time.Time, now time.Time) (time.Duration, bool) {
	if then.IsZero() {
		return 0, false
	}

	if now.Before(then) {
		now = then
	}

	return now.Sub(then), true
}
