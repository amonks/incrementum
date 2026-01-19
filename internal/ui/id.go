package ui

import (
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	ansiBold  = "\x1b[1m"
	ansiCyan  = "\x1b[36m"
	ansiReset = "\x1b[0m"
)

// HighlightID returns an ID with its unique prefix highlighted.
func HighlightID(id string, prefixLen int) string {
	if id == "" {
		return id
	}

	if prefixLen <= 0 || prefixLen > len(id) {
		return id
	}

	if !ansiEnabled() {
		return id
	}

	prefix := id[:prefixLen]
	suffix := id[prefixLen:]
	return ansiBold + ansiCyan + prefix + ansiReset + suffix
}

func ansiEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// UniqueIDPrefixLengths returns the shortest unique prefix length for each ID.
func UniqueIDPrefixLengths(ids []string) map[string]int {
	uniqueIDs := make([]string, 0, len(ids))
	seen := make(map[string]bool)
	for _, id := range ids {
		idLower := strings.ToLower(id)
		if idLower == "" || seen[idLower] {
			continue
		}
		seen[idLower] = true
		uniqueIDs = append(uniqueIDs, idLower)
	}

	lengths := make(map[string]int, len(uniqueIDs))
	for _, id := range uniqueIDs {
		lengths[id] = uniquePrefixLength(id, uniqueIDs)
	}

	return lengths
}

func uniquePrefixLength(id string, ids []string) int {
	for length := 1; length <= len(id); length++ {
		prefix := id[:length]
		unique := true
		for _, other := range ids {
			if other == id {
				continue
			}
			if strings.HasPrefix(other, prefix) {
				unique = false
				break
			}
		}
		if unique {
			return length
		}
	}

	return len(id)
}
