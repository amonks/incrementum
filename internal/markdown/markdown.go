package markdown

import (
	"strings"
	"sync"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

var (
	rendererMu sync.Mutex
	renderers  = map[int]*glamour.TermRenderer{}
)

// Render formats markdown text for terminal output.
func Render(width, indent int, input []byte) []byte {
	if len(input) == 0 {
		return nil
	}
	value := internalstrings.NormalizeNewlines(string(input))
	value = internalstrings.TrimTrailingNewlines(value)
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if width < 1 {
		width = 1
	}
	if indent < 0 {
		indent = 0
	}
	renderWidth := width - indent
	if renderWidth < 1 {
		renderWidth = 1
	}

	renderer := markdownRenderer(renderWidth)
	rendered := value
	if renderer != nil {
		formatted, err := renderer.Render(value)
		if err == nil {
			rendered = formatted
		}
	}
	rendered = internalstrings.TrimTrailingNewlines(rendered)
	if strings.TrimSpace(rendered) == "" {
		return nil
	}
	if indent <= 0 {
		return []byte(rendered)
	}
	return []byte(indentBlock(rendered, indent))
}

func markdownRenderer(width int) *glamour.TermRenderer {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	if cached, ok := renderers[width]; ok {
		return cached
	}
	style := styles.ASCIIStyleConfig
	style.Item.BlockPrefix = "- "
	style.ImageText.Format = "Image: {{.text}} ->"
	created, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	renderers[width] = created
	return created
}

func indentBlock(value string, spaces int) string {
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
