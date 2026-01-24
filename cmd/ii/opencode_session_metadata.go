package main

import (
	"encoding/json"
	"fmt"
)

type opencodeSessionMetadata struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}

type opencodeSessionList struct {
	Sessions []opencodeSessionMetadata `json:"sessions"`
}

func decodeOpencodeSessionList(data []byte) ([]opencodeSessionMetadata, error) {
	var sessions []opencodeSessionMetadata
	if err := json.Unmarshal(data, &sessions); err == nil {
		return sessions, nil
	}

	var envelope opencodeSessionList
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode opencode session list: %w", err)
	}
	return envelope.Sessions, nil
}
