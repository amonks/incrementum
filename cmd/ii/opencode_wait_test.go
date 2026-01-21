package main

import (
	"errors"
	"testing"
)

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

func TestPollOpencodeSessionStopsWhenCompleted(t *testing.T) {
	calls := 0
	list := func() ([]opencodeSessionMetadata, error) {
		calls++
		if calls == 1 {
			return []opencodeSessionMetadata{{ID: "sess-3", Status: "active"}}, nil
		}
		exitCode := 0
		return []opencodeSessionMetadata{{ID: "sess-3", Status: "completed", ExitCode: &exitCode}}, nil
	}

	session, err := pollOpencodeSession(list, "sess-3", 0)
	if err != nil {
		t.Fatalf("poll session: %v", err)
	}
	if session.Status != "completed" {
		t.Fatalf("expected completed status, got %q", session.Status)
	}
	if calls != 2 {
		t.Fatalf("expected 2 polls, got %d", calls)
	}
}

func TestPollOpencodeSessionErrorsWhenMissing(t *testing.T) {
	list := func() ([]opencodeSessionMetadata, error) {
		return []opencodeSessionMetadata{{ID: "other", Status: "active"}}, nil
	}

	_, err := pollOpencodeSession(list, "missing", 0)
	if err == nil {
		t.Fatal("expected error for missing session")
	}
	if !errors.Is(err, errOpencodeSessionMissing) {
		t.Fatalf("expected missing session error, got %v", err)
	}
}
