package job

import (
	"strings"
	"unicode"
)

func normalizeCommitMessage(message string) string {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	message = strings.ReplaceAll(message, "\r", "\n")
	message = strings.TrimRight(message, "\n")
	if message == "" {
		return ""
	}
	lines := strings.Split(message, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}
	message = strings.Join(lines, "\n")
	return trimLeadingBlankLines(message)
}

func trimLeadingBlankLines(message string) string {
	lines := strings.Split(message, "\n")
	start := 0
	for start < len(lines) {
		if strings.TrimSpace(lines[start]) != "" {
			break
		}
		start++
	}
	if start >= len(lines) {
		return ""
	}
	return strings.Join(lines[start:], "\n")
}
