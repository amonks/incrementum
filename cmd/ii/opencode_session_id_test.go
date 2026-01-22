package main

import (
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestResolveOpencodeSessionIDPrefersStoredID(t *testing.T) {
	stored := workspace.OpencodeSession{ID: "sess-123"}

	resolved := resolveOpencodeSessionID("sess", stored)

	if resolved != "sess-123" {
		t.Fatalf("expected stored id sess-123, got %q", resolved)
	}
}

func TestResolveOpencodeSessionIDFallsBackToInput(t *testing.T) {
	stored := workspace.OpencodeSession{}

	resolved := resolveOpencodeSessionID("sess-999", stored)

	if resolved != "sess-999" {
		t.Fatalf("expected input id sess-999, got %q", resolved)
	}
}
