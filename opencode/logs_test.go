package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogSnapshot(t *testing.T) {
	root := t.TempDir()
	store, err := OpenWithOptions(Options{
		StateDir:    t.TempDir(),
		StorageRoot: root,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

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

	snapshot, err := store.LogSnapshot(sessionID)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if snapshot != "line one\nline two\n" {
		t.Fatalf("expected snapshot %q, got %q", "line one\nline two\n", snapshot)
	}
}

func TestTailStreamsUpdates(t *testing.T) {
	root := t.TempDir()
	store, err := OpenWithOptions(Options{
		StateDir:    t.TempDir(),
		StorageRoot: root,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	repoPath := "/tmp/opencode-tail"
	sessionID := "ses_tail"
	if _, err := store.CreateSession(repoPath, sessionID, "Prompt", time.Now()); err != nil {
		t.Fatalf("create session: %v", err)
	}

	messageDir := filepath.Join(root, "storage", "message", sessionID)
	partDir := filepath.Join(root, "storage", "part", "msg_tail")

	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		t.Fatalf("create message dir: %v", err)
	}
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatalf("create part dir: %v", err)
	}

	writeOpencodeJSON(t, filepath.Join(messageDir, "msg_tail.json"), map[string]any{
		"id":        "msg_tail",
		"sessionID": sessionID,
		"role":      "assistant",
		"time": map[string]any{
			"created": int64(1000),
		},
	})
	writeOpencodeJSON(t, filepath.Join(partDir, "prt_first.json"), map[string]any{
		"id":        "prt_first",
		"sessionID": sessionID,
		"messageID": "msg_tail",
		"type":      "text",
		"text":      "first\n",
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var output bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- store.Tail(ctx, repoPath, sessionID, &output, 5*time.Millisecond)
	}()

	time.Sleep(10 * time.Millisecond)
	writeOpencodeJSON(t, filepath.Join(partDir, "prt_second.json"), map[string]any{
		"id":        "prt_second",
		"sessionID": sessionID,
		"messageID": "msg_tail",
		"type":      "text",
		"text":      "second\n",
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		if strings.Contains(output.String(), "second\n") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected tail to include appended content, got %q", output.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !strings.Contains(output.String(), "first\n") {
		t.Fatalf("expected tail to include initial content, got %q", output.String())
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("tail error: %v", err)
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
