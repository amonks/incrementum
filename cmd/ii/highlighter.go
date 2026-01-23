package main

import "strings"

func logHighlighter(prefixLengths map[string]int, highlight func(string, int) string) func(string) string {
	if prefixLengths == nil {
		prefixLengths = map[string]int{}
	}
	return func(id string) string {
		if id == "" {
			return id
		}
		prefixLen, ok := prefixLengths[strings.ToLower(id)]
		if !ok {
			return highlight(id, 0)
		}
		return highlight(id, prefixLen)
	}
}
