package main

import "github.com/amonks/incrementum/workspace"

func resolveOpencodeSessionID(input string, stored workspace.OpencodeSession) string {
	if stored.ID != "" {
		return stored.ID
	}
	return input
}
