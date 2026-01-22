package main

import "testing"

func TestSessionEmptyListMessageNoSessions(t *testing.T) {
	message := sessionEmptyListMessage(0, "", false)
	if message != "No sessions found." {
		t.Fatalf("expected no sessions message, got %q", message)
	}
}

func TestSessionEmptyListMessageForStatusFilter(t *testing.T) {
	message := sessionEmptyListMessage(2, "Completed", false)
	if message != "No sessions found with status completed." {
		t.Fatalf("expected status message, got %q", message)
	}
}

func TestSessionEmptyListMessageSuggestsAll(t *testing.T) {
	message := sessionEmptyListMessage(3, "", false)
	if message != "No active sessions found. Use --all to include completed/failed sessions." {
		t.Fatalf("expected --all hint, got %q", message)
	}
}

func TestOpencodeEmptyListMessageNoSessions(t *testing.T) {
	message := opencodeEmptyListMessage(0, false)
	if message != "No opencode sessions found." {
		t.Fatalf("expected opencode empty message, got %q", message)
	}
}

func TestOpencodeEmptyListMessageSuggestsAll(t *testing.T) {
	message := opencodeEmptyListMessage(2, false)
	if message != "No active opencode sessions found. Use --all to include inactive sessions." {
		t.Fatalf("expected opencode --all hint, got %q", message)
	}
}
