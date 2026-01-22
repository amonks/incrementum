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

func TestTodoEmptyListMessageNoTodos(t *testing.T) {
	message := todoEmptyListMessage(0, "", false, false, false, false)
	if message != "No todos found." {
		t.Fatalf("expected no todos message, got %q", message)
	}
}

func TestTodoEmptyListMessageForStatusFilter(t *testing.T) {
	message := todoEmptyListMessage(2, "Done", false, false, false, false)
	if message != "No todos found with status done." {
		t.Fatalf("expected status message, got %q", message)
	}
}

func TestTodoEmptyListMessageSuggestsAll(t *testing.T) {
	message := todoEmptyListMessage(2, "", false, false, true, false)
	if message != "No todos found. Use --all to include done todos." {
		t.Fatalf("expected --all hint, got %q", message)
	}
}

func TestTodoEmptyListMessageSuggestsTombstones(t *testing.T) {
	message := todoEmptyListMessage(2, "", true, false, false, true)
	if message != "No todos found. Use --tombstones to include deleted todos." {
		t.Fatalf("expected --tombstones hint, got %q", message)
	}
}

func TestTodoEmptyListMessageSuggestsAllAndTombstones(t *testing.T) {
	message := todoEmptyListMessage(3, "", false, false, true, true)
	if message != "No todos found. Use --all to include done todos. Use --tombstones to include deleted todos." {
		t.Fatalf("expected combined hint, got %q", message)
	}
}
