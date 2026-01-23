package todo

import (
	"time"

	"github.com/amonks/incrementum/internal/ids"
)

// GenerateID creates a unique 8-character alphanumeric ID from a title and timestamp.
// The ID is derived from SHA-256 hash of the title concatenated with the timestamp.
func GenerateID(title string, timestamp time.Time) string {
	input := title + timestamp.Format(time.RFC3339Nano)
	return ids.Generate(input, 8)
}
