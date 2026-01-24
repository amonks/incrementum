package main

import "testing"

func TestDecodeOpencodeSessionListSupportsArray(t *testing.T) {
	data := []byte(`[{"id":"sess-1","status":"active"}]`)

	sessions, err := decodeOpencodeSessionList(data)
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

func TestDecodeOpencodeSessionListSupportsEnvelope(t *testing.T) {
	data := []byte(`{"sessions":[{"id":"sess-2","status":"completed","exit_code":2,"duration_seconds":12}]}`)

	sessions, err := decodeOpencodeSessionList(data)
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
