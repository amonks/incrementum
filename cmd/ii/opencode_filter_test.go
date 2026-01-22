package main

import (
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestFilterOpencodeSessionsForListDefaultsToActive(t *testing.T) {
	sessions := []workspace.OpencodeSession{
		{ID: "active", Status: workspace.OpencodeSessionActive},
		{ID: "completed", Status: workspace.OpencodeSessionCompleted},
		{ID: "failed", Status: workspace.OpencodeSessionFailed},
		{ID: "killed", Status: workspace.OpencodeSessionKilled},
	}

	filtered := filterOpencodeSessionsForList(sessions, false)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(filtered))
	}
	if filtered[0].ID != "active" {
		t.Fatalf("expected active session, got %q", filtered[0].ID)
	}
}

func TestFilterOpencodeSessionsForListWithAll(t *testing.T) {
	sessions := []workspace.OpencodeSession{
		{ID: "active", Status: workspace.OpencodeSessionActive},
		{ID: "completed", Status: workspace.OpencodeSessionCompleted},
	}

	filtered := filterOpencodeSessionsForList(sessions, true)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(filtered))
	}
}

func TestFilterOpencodeSessionsForListPreservesInputOrder(t *testing.T) {
	sessions := []workspace.OpencodeSession{
		{ID: "active", Status: workspace.OpencodeSessionActive},
		{ID: "completed", Status: workspace.OpencodeSessionCompleted},
		{ID: "failed", Status: workspace.OpencodeSessionFailed},
	}

	originalIDs := make([]string, len(sessions))
	for i, session := range sessions {
		originalIDs[i] = session.ID
	}

	_ = filterOpencodeSessionsForList(sessions, false)

	for i, session := range sessions {
		if session.ID != originalIDs[i] {
			t.Fatalf("expected input slice to remain unchanged at %d, got %q", i, session.ID)
		}
	}
}
