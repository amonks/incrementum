package main

import (
	"fmt"
	"strings"
)

func sessionEmptyListMessage(total int, status string, includeAll bool) string {
	if total == 0 {
		return "No sessions found."
	}

	status = strings.TrimSpace(status)
	if status != "" {
		return fmt.Sprintf("No sessions found with status %s.", strings.ToLower(status))
	}

	if !includeAll {
		return "No active sessions found. Use --all to include completed/failed sessions."
	}

	return "No sessions found."
}

func opencodeEmptyListMessage(total int, includeAll bool) string {
	if total == 0 {
		return "No opencode sessions found."
	}

	if !includeAll {
		return "No active opencode sessions found. Use --all to include inactive sessions."
	}

	return "No opencode sessions found."
}
