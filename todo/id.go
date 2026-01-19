package todo

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"
	"time"
)

// GenerateID creates a unique 8-character alphanumeric ID from a title and timestamp.
// The ID is derived from SHA-256 hash of the title concatenated with the timestamp.
func GenerateID(title string, timestamp time.Time) string {
	input := title + timestamp.Format(time.RFC3339Nano)
	hash := sha256.Sum256([]byte(input))

	// Use base32 encoding (alphanumeric, case-insensitive)
	// Take first 8 characters and lowercase for readability
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	return strings.ToLower(encoded[:8])
}
