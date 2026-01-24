package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	internalopencode "github.com/amonks/incrementum/internal/opencode"
)

func TestOpencodeLogSnapshot(t *testing.T) {
	root := t.TempDir()
	storage := internalopencode.Storage{Root: root}
	sessionID := "ses_logtest"
	messageDir := filepath.Join(root, "storage", "message", sessionID)
	partDir := filepath.Join(root, "storage", "part", "msg_log")

	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		t.Fatalf("create message dir: %v", err)
	}
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatalf("create part dir: %v", err)
	}

	writeOpencodeJSON(t, filepath.Join(messageDir, "msg_log.json"), map[string]any{
		"id":        "msg_log",
		"sessionID": sessionID,
		"role":      "assistant",
		"time": map[string]any{
			"created": int64(1000),
		},
	})
	writeOpencodeJSON(t, filepath.Join(partDir, "prt_log.json"), map[string]any{
		"id":        "prt_log",
		"sessionID": sessionID,
		"messageID": "msg_log",
		"type":      "text",
		"text":      "line one\nline two\n",
	})

	snapshot, err := opencodeLogSnapshot(storage, sessionID)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snapshot != "line one\nline two\n" {
		t.Fatalf("expected snapshot %q, got %q", "line one\\nline two\\n", snapshot)
	}
}

func writeOpencodeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
