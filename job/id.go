package job

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"
	"time"
)

// GenerateID creates a unique ID from a todo ID and timestamp.
func GenerateID(todoID string, timestamp time.Time) string {
	input := todoID + timestamp.Format(time.RFC3339Nano)
	hash := sha256.Sum256([]byte(input))
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	return strings.ToLower(encoded[:10])
}
