package job

import (
	"time"

	"github.com/amonks/incrementum/internal/ids"
)

// GenerateID creates a unique ID from a todo ID and timestamp.
func GenerateID(todoID string, timestamp time.Time) string {
	input := todoID + timestamp.Format(time.RFC3339Nano)
	return ids.Generate(input, ids.DefaultLength)
}
