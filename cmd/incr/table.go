package main

import "strings"

func formatTable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(stripANSICodes(header))
	}

	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if displayLen := len(stripANSICodes(cell)); displayLen > widths[i] {
				widths[i] = displayLen
			}
		}
	}

	var builder strings.Builder
	writeRow := func(row []string) {
		for i, cell := range row {
			cellWidth := len(stripANSICodes(cell))
			builder.WriteString(cell)
			if i == len(row)-1 {
				builder.WriteByte('\n')
				continue
			}
			padding := widths[i] - cellWidth
			builder.WriteString(strings.Repeat(" ", padding+2))
		}
	}

	writeRow(headers)
	for _, row := range rows {
		writeRow(row)
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
