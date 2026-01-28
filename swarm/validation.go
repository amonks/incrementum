package swarm

import (
	"fmt"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

func requiredTrimmed(value, field string) (string, error) {
	trimmed := internalstrings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	return trimmed, nil
}
