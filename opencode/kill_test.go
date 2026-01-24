package opencode

import "testing"

func TestDecodeSessionMetadataSupportsSessionEnvelope(t *testing.T) {
	data := []byte(`{"session":{"id":"sess-1","status":"killed","exit_code":137,"duration_seconds":12}}`)

	session, err := decodeSessionMetadata(data)
	if err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.ID != "sess-1" {
		t.Fatalf("expected session id sess-1, got %q", session.ID)
	}
	if session.Status != "killed" {
		t.Fatalf("expected status killed, got %q", session.Status)
	}
	if session.ExitCode == nil || *session.ExitCode != 137 {
		t.Fatalf("expected exit code 137, got %v", session.ExitCode)
	}
	if session.DurationSeconds != 12 {
		t.Fatalf("expected duration 12, got %d", session.DurationSeconds)
	}
}

func TestDecodeSessionMetadataSupportsFlatObject(t *testing.T) {
	data := []byte(`{"id":"sess-2","status":"completed"}`)

	session, err := decodeSessionMetadata(data)
	if err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.ID != "sess-2" {
		t.Fatalf("expected session id sess-2, got %q", session.ID)
	}
	if session.Status != "completed" {
		t.Fatalf("expected status completed, got %q", session.Status)
	}
}

func TestDecodeSessionListSupportsArray(t *testing.T) {
	data := []byte(`[{"id":"sess-1","status":"active"}]`)

	sessions, err := decodeSessionList(data)
	if err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Fatalf("expected session id sess-1, got %q", sessions[0].ID)
	}
	if sessions[0].Status != "active" {
		t.Fatalf("expected status active, got %q", sessions[0].Status)
	}
}

func TestDecodeSessionListSupportsEnvelope(t *testing.T) {
	data := []byte(`{"sessions":[{"id":"sess-2","status":"completed","exit_code":2,"duration_seconds":12}]}`)

	sessions, err := decodeSessionList(data)
	if err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Fatalf("expected session id sess-2, got %q", sessions[0].ID)
	}
	if sessions[0].ExitCode == nil || *sessions[0].ExitCode != 2 {
		t.Fatalf("expected exit code 2, got %v", sessions[0].ExitCode)
	}
	if sessions[0].DurationSeconds != 12 {
		t.Fatalf("expected duration 12, got %d", sessions[0].DurationSeconds)
	}
}
