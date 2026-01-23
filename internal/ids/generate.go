package ids

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"
)

// Generate creates a deterministic, lowercase base32 ID derived from input.
func Generate(input string, length int) string {
	hash := sha256.Sum256([]byte(input))
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	if length <= 0 {
		return ""
	}
	if length > len(encoded) {
		length = len(encoded)
	}
	return strings.ToLower(encoded[:length])
}
