package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorageSessionLogText(t *testing.T) {
	root := t.TempDir()
	storage := Storage{Root: root}
	sessionID := "ses_test123"

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
	writeJSON(t, filepath.Join(partAssistantDir, "prt_100_tool.json"), map[string]any{
		"id":        "prt_100_tool",
		"sessionID": sessionID,
		"messageID": "msg_assistant",
		"type":      "tool",
		"tool":      "read",
		"state": map[string]any{
			"output": "Tool output\n",
		},
	})
	writeJSON(t, filepath.Join(partAssistantDir, "prt_200_text.json"), map[string]any{
		"id":        "prt_200_text",
		"sessionID": sessionID,
		"messageID": "msg_assistant",
		"type":      "text",
		"text":      "Goodbye\n",
	})

	logText, err := storage.SessionLogText(sessionID)
	if err != nil {
		t.Fatalf("read session log: %v", err)
	}

	expected := "Hello\nTool output\nGoodbye\n"
	if logText != expected {
		t.Fatalf("expected log %q, got %q", expected, logText)
	}
}

func TestStorageFindSessionForRunMatchesPrompt(t *testing.T) {
	root := t.TempDir()
	storage := Storage{Root: root}
	repoPath := filepath.Join(root, "repo")
	projectID := "proj_123"

	projectDir := filepath.Join(root, "storage", "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}
	writeJSON(t, filepath.Join(projectDir, projectID+".json"), map[string]any{
		"id":       projectID,
		"worktree": repoPath,
	})

	sessionDir := filepath.Join(root, "storage", "session", projectID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("create session dir: %v", err)
	}

	writeSession := func(sessionID, prompt string, created int64) {
		writeJSON(t, filepath.Join(sessionDir, sessionID+".json"), map[string]any{
			"id":        sessionID,
			"projectID": projectID,
			"directory": repoPath,
			"time": map[string]any{
				"created": created,
			},
		})

		messageDir := filepath.Join(root, "storage", "message", sessionID)
		partDir := filepath.Join(root, "storage", "part", "msg_"+sessionID)
		if err := os.MkdirAll(messageDir, 0o755); err != nil {
			t.Fatalf("create message dir: %v", err)
		}
		if err := os.MkdirAll(partDir, 0o755); err != nil {
			t.Fatalf("create part dir: %v", err)
		}

		messageID := "msg_" + sessionID
		writeJSON(t, filepath.Join(messageDir, messageID+".json"), map[string]any{
			"id":        messageID,
			"sessionID": sessionID,
			"role":      "user",
			"time": map[string]any{
				"created": created,
			},
		})
		writeJSON(t, filepath.Join(partDir, "prt_"+sessionID+".json"), map[string]any{
			"id":        "prt_" + sessionID,
			"sessionID": sessionID,
			"messageID": messageID,
			"type":      "text",
			"text":      prompt,
		})
	}

	writeSession("ses_good", "Run the prompt", 1000)
	writeSession("ses_other", "Other prompt", 1200)

	startedAt := time.UnixMilli(900)
	found, err := storage.FindSessionForRun(repoPath, startedAt, "Run the prompt")
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if found.ID != "ses_good" {
		t.Fatalf("expected session ses_good, got %q", found.ID)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
