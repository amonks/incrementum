package opencode

import "testing"

func TestFilterSessionsForListDefaultsToActive(t *testing.T) {
	sessions := []OpencodeSession{
		{ID: "active", Status: OpencodeSessionActive},
		{ID: "completed", Status: OpencodeSessionCompleted},
		{ID: "failed", Status: OpencodeSessionFailed},
		{ID: "killed", Status: OpencodeSessionKilled},
	}

	filtered := FilterSessionsForList(sessions, false)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(filtered))
	}
	if filtered[0].ID != "active" {
		t.Fatalf("expected active session, got %q", filtered[0].ID)
	}
}

func TestFilterSessionsForListWithAll(t *testing.T) {
	sessions := []OpencodeSession{
		{ID: "active", Status: OpencodeSessionActive},
		{ID: "completed", Status: OpencodeSessionCompleted},
	}

	filtered := FilterSessionsForList(sessions, true)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(filtered))
	}
}

func TestFilterSessionsForListPreservesInputOrder(t *testing.T) {
	sessions := []OpencodeSession{
		{ID: "active", Status: OpencodeSessionActive},
		{ID: "completed", Status: OpencodeSessionCompleted},
		{ID: "failed", Status: OpencodeSessionFailed},
	}

	originalIDs := make([]string, len(sessions))
	for i, session := range sessions {
		originalIDs[i] = session.ID
	}

	_ = FilterSessionsForList(sessions, false)

	for i, session := range sessions {
		if session.ID != originalIDs[i] {
			t.Fatalf("expected input slice to remain unchanged at %d, got %q", i, session.ID)
		}
	}
}
