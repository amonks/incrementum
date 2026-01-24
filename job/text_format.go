package job

import (
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/muesli/reflow/wordwrap"
)

const (
	lineWidth         = 80
	documentIndent    = 4
	subdocumentIndent = 8
)

func wrapLines(value string, width int) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return wordwrap.String(value, width)
}

func reflowParagraphs(value string, width int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	paragraphs := splitParagraphs(value)
	wrapped := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		normalized := internalstrings.NormalizeWhitespace(paragraph)
		if normalized == "" {
			continue
		}
		wrapped = append(wrapped, wordwrap.String(normalized, width))
	}
	return strings.Join(wrapped, "\n\n")
}

func splitParagraphs(value string) []string {
	lines := strings.Split(value, "\n")
	var paragraphs []string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(current, " "))
		current = nil
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return paragraphs
}

func indentBlock(value string, spaces int) string {
	value = strings.TrimRight(value, "\r\n")
	if spaces <= 0 {
		return value
	}
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func reflowIndentedText(value string, width int, baseIndent int) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.TrimRight(value, "\n")
	if strings.TrimSpace(value) == "" {
		return indentBlock("-", baseIndent)
	}

	lines := strings.Split(value, "\n")
	var out []string
	for i := 0; i < len(lines); {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			out = append(out, strings.Repeat(" ", baseIndent))
			i++
			continue
		}
		indent := leadingSpaces(line)
		var parts []string
		for i < len(lines) {
			line = lines[i]
			if strings.TrimSpace(line) == "" {
				break
			}
			if leadingSpaces(line) != indent {
				break
			}
			parts = append(parts, strings.TrimSpace(line[indent:]))
			i++
		}
		normalized := internalstrings.NormalizeWhitespace(strings.Join(parts, " "))
		if normalized == "" {
			out = append(out, strings.Repeat(" ", baseIndent+indent)+"-")
			continue
		}
		wrapWidth := width - baseIndent - indent
		if wrapWidth < 1 {
			wrapWidth = 1
		}
		wrapped := wordwrap.String(normalized, wrapWidth)
		wrapped = indentBlock(wrapped, baseIndent+indent)
		out = append(out, strings.Split(wrapped, "\n")...)
	}
	return strings.Join(out, "\n")
}

func leadingSpaces(value string) int {
	count := 0
	for _, char := range value {
		if char != ' ' {
			break
		}
		count++
	}
	return count
}
