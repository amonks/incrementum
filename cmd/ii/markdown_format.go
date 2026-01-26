package main

import (
	"strings"

	jobpkg "github.com/amonks/incrementum/job"
)

func renderMarkdownOrDash(value string, width int) string {
	formatted := jobpkg.RenderMarkdown(value, width)
	if strings.TrimSpace(formatted) == "" {
		return "-"
	}
	return formatted
}
