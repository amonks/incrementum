package validation

import "strings"

// FormatValidValues joins string-like values for error messages.
func FormatValidValues[T ~string](values []T) string {
	formatted := make([]string, 0, len(values))
	for _, value := range values {
		formatted = append(formatted, string(value))
	}
	return strings.Join(formatted, ", ")
}
