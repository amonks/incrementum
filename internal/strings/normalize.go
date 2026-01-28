package strings

import (
	"strings"
	"unicode"
)

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

// NormalizeNewlines replaces CRLF and CR with LF.
func NormalizeNewlines(value string) string {
	if value == "" {
		return value
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

// TrimTrailingCarriageReturn removes a trailing carriage return if present.
func TrimTrailingCarriageReturn(value string) string {
	return strings.TrimSuffix(value, "\r")
}

// TrimTrailingNewlines removes trailing CR/LF characters.
func TrimTrailingNewlines(value string) string {
	return strings.TrimRight(value, "\r\n")
}

// TrimTrailingWhitespace removes trailing Unicode whitespace characters.
func TrimTrailingWhitespace(value string) string {
	return strings.TrimRightFunc(value, unicode.IsSpace)
}

// TrimTrailingSlash removes trailing '/' characters.
func TrimTrailingSlash(value string) string {
	return strings.TrimRight(value, "/")
}

// LeadingSpaces counts leading ASCII space characters.
func LeadingSpaces(value string) int {
	count := 0
	for _, char := range value {
		if char != ' ' {
			break
		}
		count++
	}
	return count
}
