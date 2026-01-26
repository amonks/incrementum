package main

import (
	"strings"

	jobpkg "github.com/amonks/incrementum/job"
)

func renderMarkdownOrDash(value string, width int) string {
	if width < 1 {
		width = 1
	}
	formatted := jobpkg.RenderMarkdown(value, width)
	if strings.TrimSpace(formatted) == "" {
		return "-"
	}
	return formatted
}
