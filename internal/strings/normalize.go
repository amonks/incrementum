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

// NormalizeLower returns the input lowercased.
func NormalizeLower(value string) string {
	return strings.ToLower(value)
}

// NormalizeLowerTrimSpace trims surrounding whitespace and lowercases the input.
func NormalizeLowerTrimSpace(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// TrimTrailingCarriageReturn removes a trailing carriage return if present.
func TrimTrailingCarriageReturn(value string) string {
	return strings.TrimSuffix(value, "\r")
}

// TrimTrailingNewlines removes trailing CR/LF characters.
func TrimTrailingNewlines(value string) string {
	return strings.TrimRight(value, "\r\n")
}
