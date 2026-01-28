package job

import (
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

func formatOpencodeText(event opencodeRenderedEvent) []string {
	if event.Inline != "" {
		line := strings.TrimSpace(strings.Join([]string{event.Label, event.Inline}, " "))
		if internalstrings.IsBlank(line) {
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

func formatPlainBody(body string, indent int) string {
	body = normalizeLogBody(body)
	if strings.TrimSpace(body) == "-" {
		return IndentBlock(body, indent)
	}
	rendered := ReflowParagraphs(body, wrapWidthFor(lineWidth, indent))
	if internalstrings.IsBlank(rendered) {
		return IndentBlock("-", indent)
	}
	return IndentBlock(rendered, indent)
}
