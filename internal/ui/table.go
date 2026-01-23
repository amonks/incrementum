package ui

import (
	"strings"
	"unicode/utf8"
)

const tableCellMaxWidth = 50
const tableCellEllipsis = "..."

// TableBuilder collects rows and renders a formatted table.
type TableBuilder struct {
	headers []string
	rows    [][]string
}

// NewTableBuilder returns a builder with preallocated rows.
func NewTableBuilder(headers []string, capacity int) *TableBuilder {
	return &TableBuilder{headers: headers, rows: make([][]string, 0, capacity)}
}

// AddRow appends a row to the table.
func (builder *TableBuilder) AddRow(row []string) {
	builder.rows = append(builder.rows, row)
}

// String renders the table output.
func (builder *TableBuilder) String() string {
	return FormatTable(builder.headers, builder.rows)
}

// FormatTable renders headers and rows as an aligned table.
func FormatTable(headers []string, rows [][]string) string {
	normalizedHeaders := make([]string, len(headers))
	for i, header := range headers {
		normalizedHeaders[i] = normalizeTableCell(header)
	}

	normalizedRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		normalizedRow := make([]string, len(row))
		for i, cell := range row {
			normalizedRow[i] = normalizeTableCell(cell)
		}
		normalizedRows = append(normalizedRows, normalizedRow)
	}

	widths := make([]int, len(normalizedHeaders))
	for i, header := range normalizedHeaders {
		widths[i] = displayWidth(header)
	}

	for _, row := range normalizedRows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if displayLen := displayWidth(cell); displayLen > widths[i] {
				widths[i] = displayLen
			}
		}
	}

	var builder strings.Builder
	writeRow := func(row []string) {
		for i, cell := range row {
			cellWidth := displayWidth(cell)
			builder.WriteString(cell)
			if i == len(row)-1 {
				builder.WriteByte('\n')
				continue
			}
			padding := widths[i] - cellWidth
			builder.WriteString(strings.Repeat(" ", padding+2))
		}
	}

	writeRow(normalizedHeaders)
	for _, row := range normalizedRows {
		writeRow(row)
	}

	return builder.String()
}

// TruncateTableCell limits cell width while preserving visible characters.
func TruncateTableCell(value string) string {
	value = normalizeTableCell(value)
	if displayWidth(value) <= tableCellMaxWidth {
		return value
	}

	max := tableCellMaxWidth - displayWidth(tableCellEllipsis)
	if max <= 0 {
		return tableCellEllipsis
	}
	return truncateVisible(value, max) + tableCellEllipsis
}

func displayWidth(value string) int {
	return utf8.RuneCountInString(stripANSICodes(value))
}

func normalizeTableCell(value string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(value)
}

func truncateVisible(value string, max int) string {
	if max <= 0 {
		return ""
	}

	var builder strings.Builder
	visible := 0
	for i := 0; i < len(value); {
		if value[i] == '\x1b' {
			end := i + 1
			if end < len(value) && value[end] == '[' {
				end++
				for end < len(value) && value[end] != 'm' {
					end++
				}
				if end < len(value) {
					end++
				}
				builder.WriteString(value[i:end])
				i = end
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 1 {
			if visible >= max {
				break
			}
			builder.WriteByte(value[i])
			visible++
			i++
			continue
		}
		if visible >= max {
			break
		}
		builder.WriteRune(r)
		visible++
		i += size
	}
	return builder.String()
}

func stripANSICodes(input string) string {
	var builder strings.Builder
	inEscape := false
	for i := 0; i < len(input); i++ {
		char := input[i]
		if inEscape {
			if char == 'm' {
				inEscape = false
			}
			continue
		}
		if char == '\x1b' {
			inEscape = true
			continue
		}
		builder.WriteByte(char)
	}
	return builder.String()
}
