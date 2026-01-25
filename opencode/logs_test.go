package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLogSnapshot(t *testing.T) {
	root := t.TempDir()
	store, err := OpenWithOptions(Options{
		StateDir:    t.TempDir(),
		StorageRoot: root,
		EventsDir:   filepath.Join(root, "events"),
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	sessionID := "ses_logtest"
	logDir := filepath.Join(root, "events")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("create events dir: %v", err)
	}
	logPath := filepath.Join(logDir, sessionID+".sse")
	if err := os.WriteFile(logPath, []byte("event: message\ndata: line one\ndata: line two\n\n"), 0o644); err != nil {
		t.Fatalf("write event log: %v", err)
	}

	snapshot, err := store.LogSnapshot(sessionID)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snapshot != "event: message\ndata: line one\ndata: line two\n\n" {
		t.Fatalf("expected snapshot to match event log, got %q", snapshot)
	}
}

func TestTranscriptSnapshotIncludesToolOutput(t *testing.T) {
	root := t.TempDir()
	store, err := OpenWithOptions(Options{
		StateDir:    t.TempDir(),
		StorageRoot: root,
		EventsDir:   filepath.Join(root, "events"),
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	sessionID := "ses_transcript"
	messageDir := filepath.Join(root, "storage", "message", sessionID)
	partUserDir := filepath.Join(root, "storage", "part", "msg_user")
	partAssistantDir := filepath.Join(root, "storage", "part", "msg_assistant")

	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		t.Fatalf("create message dir: %v", err)
	}
	if err := os.MkdirAll(partUserDir, 0o755); err != nil {
		t.Fatalf("create user part dir: %v", err)
	}
	if err := os.MkdirAll(partAssistantDir, 0o755); err != nil {
		t.Fatalf("create assistant part dir: %v", err)
	}

	writeJSON(t, filepath.Join(messageDir, "msg_user.json"), map[string]any{
		"id":        "msg_user",
		"sessionID": sessionID,
		"role":      "user",
		"time": map[string]any{
			"created": int64(1000),
		},
	})
	writeJSON(t, filepath.Join(messageDir, "msg_assistant.json"), map[string]any{
		"id":        "msg_assistant",
		"sessionID": sessionID,
		"role":      "assistant",
		"time": map[string]any{
			"created": int64(2000),
		},
	})

	writeJSON(t, filepath.Join(partUserDir, "prt_user.json"), map[string]any{
		"id":        "prt_user",
		"sessionID": sessionID,
		"messageID": "msg_user",
		"type":      "text",
		"text":      "Hello\n",
	})
	writeJSON(t, filepath.Join(partAssistantDir, "prt_tool.json"), map[string]any{
		"id":        "prt_tool",
		"sessionID": sessionID,
		"messageID": "msg_assistant",
		"type":      "tool",
		"state": map[string]any{
			"output": map[string]any{
				"stdout": "Tool output\n",
			},
		},
	})
	writeJSON(t, filepath.Join(partAssistantDir, "prt_text.json"), map[string]any{
		"id":        "prt_text",
		"sessionID": sessionID,
		"messageID": "msg_assistant",
		"type":      "text",
		"text":      "Goodbye\n",
	})

	snapshot, err := store.TranscriptSnapshot(sessionID)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}

	expected := "Hello\nStdout:\n    Tool output\nGoodbye\n"
	if snapshot != expected {
		t.Fatalf("expected transcript %q, got %q", expected, snapshot)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
