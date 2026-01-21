package main

import (
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestDecodeOpencodeSessionMetadataSupportsSessionEnvelope(t *testing.T) {
	data := []byte(`{"session":{"id":"sess-1","status":"killed","exit_code":137,"duration_seconds":12}}`)

	session, err := decodeOpencodeSessionMetadata(data)
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

func TestDecodeOpencodeSessionMetadataSupportsFlatObject(t *testing.T) {
	data := []byte(`{"id":"sess-2","status":"completed"}`)

	session, err := decodeOpencodeSessionMetadata(data)
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

func TestResolveOpencodeKillStatusAlwaysKilled(t *testing.T) {
	metadata := opencodeSessionMetadata{Status: "completed"}

	status := resolveOpencodeKillStatus(metadata)
	if status != workspace.OpencodeSessionKilled {
		t.Fatalf("expected status killed, got %q", status)
	}
}
