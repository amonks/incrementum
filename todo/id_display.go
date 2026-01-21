package todo

import (
	"fmt"
	"strings"

	"github.com/amonks/incrementum/internal/ids"
)

// IDIndex indexes todo IDs for prefix matching and display.
type IDIndex struct {
	ids []string
}

// NewIDIndex builds an IDIndex from a slice of todos.
func NewIDIndex(todos []Todo) IDIndex {
	ids := make([]string, 0, len(todos))
	seen := make(map[string]struct{})
	for _, todo := range todos {
		id := strings.ToLower(todo.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return IDIndex{ids: ids}
}

// Resolve returns the full todo ID for a prefix.
func (index IDIndex) Resolve(prefix string) (string, error) {
	if prefix == "" {
		return "", ErrTodoNotFound
	}

	prefixLower := strings.ToLower(prefix)
	var matches []string
	for _, id := range index.ids {
		if id == prefixLower || strings.HasPrefix(id, prefixLower) {
			matches = append(matches, id)
		}
	}

	if len(matches) == 0 {
		return "", ErrTodoNotFound
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%w: %s", ErrAmbiguousTodoIDPrefix, prefix)
	}

	return matches[0], nil
}

// PrefixLengths returns the shortest unique prefix length for each ID.
func (index IDIndex) PrefixLengths() map[string]int {
	return ids.UniquePrefixLengths(index.ids)
}
