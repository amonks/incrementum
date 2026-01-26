package job

import "strings"

func formatOpencodeText(event opencodeRenderedEvent) []string {
	if event.Inline != "" {
		line := strings.TrimSpace(strings.Join([]string{event.Label, event.Inline}, " "))
		if line == "" {
			return nil
		}
		return []string{IndentBlock(line, documentIndent)}
	}
	if strings.TrimSpace(event.Label) == "" && strings.TrimSpace(event.Body) == "" {
		return nil
	}
	if strings.TrimSpace(event.Body) == "" {
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
	if strings.TrimSpace(rendered) == "" {
		return IndentBlock("-", indent)
	}
	return IndentBlock(rendered, indent)
}
