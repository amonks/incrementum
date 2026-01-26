# Internal Markdown

## Overview
The internal markdown package wraps glamour to render markdown for CLI output.

## Rendering
- `Render(width, indent int, input []byte) []byte` returns markdown rendered for terminal display.
- The renderer normalizes CRLF line endings and trims trailing newlines before rendering.
- Rendered output has trailing whitespace stripped per line.
- Width accounts for indentation; the renderer wraps to `width - indent` with a minimum of 1.
- Indentation prefixes each rendered line with the requested number of spaces.
- Rendering uses the ASCII glamour style with a `- ` list prefix and `Image: {{.text}} ->` image text.
- Renderers are cached per width for reuse.
