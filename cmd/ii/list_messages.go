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
		return "No active opencode sessions found. Use --all to include completed/failed/killed sessions."
	}

	return "No opencode sessions found."
}

func jobEmptyListMessage(total int, status string, includeAll bool) string {
	if total == 0 {
		return "No jobs found."
	}

	status = strings.TrimSpace(status)
	if status != "" {
		return fmt.Sprintf("No jobs found with status %s.", strings.ToLower(status))
	}

	if !includeAll {
		return "No active jobs found. Use --all to include completed/failed/abandoned jobs."
	}

	return "No jobs found."
}

func todoEmptyListMessage(total int, status string, includeAll bool, includeTombstones bool, hasDone bool, hasTombstones bool) string {
	if total == 0 {
		return "No todos found."
	}

	status = strings.TrimSpace(status)
	if status != "" {
		return fmt.Sprintf("No todos found with status %s.", strings.ToLower(status))
	}

	hints := make([]string, 0, 2)
	if !includeAll && hasDone {
		hints = append(hints, "Use --all to include done todos.")
	}
	if !includeTombstones && hasTombstones {
		hints = append(hints, "Use --tombstones to include deleted todos.")
	}
	if len(hints) > 0 {
		return fmt.Sprintf("No todos found. %s", strings.Join(hints, " "))
	}

	return "No todos found."
}
