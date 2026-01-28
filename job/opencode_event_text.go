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
	labelBlank := internalstrings.IsBlank(event.Label)
	bodyBlank := internalstrings.IsBlank(event.Body)
	if labelBlank && bodyBlank {
		return nil
	}
	if bodyBlank {
		return []string{formatLogLabel(event.Label, documentIndent)}
	}
	return []string{
		formatLogLabel(event.Label, documentIndent),
		formatMarkdownBody(event.Body, subdocumentIndent),
	}
}
