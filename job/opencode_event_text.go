package job

import (
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

func formatOpencodeText(event opencodeRenderedEvent) []string {
	if event.Inline != "" {
		line := internalstrings.TrimSpace(event.Label + " " + event.Inline)
		if line == "" {
			return nil
		}
		return []string{IndentBlock(line, documentIndent)}
	}
	if internalstrings.IsBlank(event.Label) && internalstrings.IsBlank(event.Body) {
		return nil
	}
	if internalstrings.IsBlank(event.Body) {
		return []string{formatLogLabel(event.Label, documentIndent)}
	}
	return []string{
		formatLogLabel(event.Label, documentIndent),
		formatMarkdownBody(event.Body, subdocumentIndent),
	}
}
