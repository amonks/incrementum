package strings

import "strings"

// NormalizeWhitespace collapses runs of whitespace into single spaces.
func NormalizeWhitespace(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
