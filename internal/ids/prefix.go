package ids

import "strings"

// UniquePrefixLengths returns the shortest unique prefix length for each ID.
func UniquePrefixLengths(ids []string) map[string]int {
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
