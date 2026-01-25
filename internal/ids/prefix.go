package ids

import (
	"strings"
	"unicode/utf8"
)

// NormalizeUniqueIDs lowercases IDs and removes duplicates or empty values.
func NormalizeUniqueIDs(ids []string) []string {
	uniqueIDs := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		idLower := normalizeID(id)
		if idLower == "" || seen[idLower] {
			continue
		}
		seen[idLower] = true
		uniqueIDs = append(uniqueIDs, idLower)
	}
	return uniqueIDs
}

func normalizeID(id string) string {
	for i := 0; i < len(id); i++ {
		b := id[i]
		if b >= utf8.RuneSelf {
			return strings.ToLower(id)
		}
		if b >= 'A' && b <= 'Z' {
			return strings.ToLower(id)
		}
	}
	return id
}

// UniquePrefixLengths returns the shortest unique prefix length for each ID.
func UniquePrefixLengths(ids []string) map[string]int {
	uniqueIDs := NormalizeUniqueIDs(ids)
	return UniquePrefixLengthsNormalized(uniqueIDs)
}

// UniquePrefixLengthsNormalized returns the shortest unique prefix length for each ID.
// It expects ids to already be normalized and de-duplicated.
func UniquePrefixLengthsNormalized(ids []string) map[string]int {
	lengths := make(map[string]int, len(ids))
	for _, id := range ids {
		lengths[id] = uniquePrefixLength(id, ids)
	}

	return lengths
}

// MatchPrefix returns the matching ID for a non-empty prefix.
// The returned match preserves the original ID casing.
func MatchPrefix(ids []string, prefix string) (string, bool, bool) {
	needle := strings.ToLower(prefix)
	var match string
	for _, id := range ids {
		idLower := strings.ToLower(id)
		if idLower != needle && !strings.HasPrefix(idLower, needle) {
			continue
		}
		if match != "" && !strings.EqualFold(match, id) {
			return "", true, true
		}
		match = id
	}

	if match == "" {
		return "", false, false
	}

	return match, true, false
}

// MatchPrefixNormalized returns the matching ID for a non-empty prefix.
// It expects ids to already be normalized and de-duplicated.
func MatchPrefixNormalized(ids []string, prefix string) (string, bool, bool) {
	needle := strings.ToLower(prefix)
	var match string
	for _, id := range ids {
		if id != needle && !strings.HasPrefix(id, needle) {
			continue
		}
		if match != "" && match != id {
			return "", true, true
		}
		match = id
	}

	if match == "" {
		return "", false, false
	}

	return match, true, false
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
