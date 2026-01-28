package job

import (
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

func formatOpencodeText(event opencodeRenderedEvent) []string {
	if event.Inline != "" {
		return []string{IndentBlock(event.Label+" "+event.Inline, documentIndent)}
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
