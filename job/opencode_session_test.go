package job

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/opencode"
)

func TestRunOpencodeSessionRecordsActiveSessionDuringRun(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create home: %v", err)
	}
	t.Setenv("HOME", home)

	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}

	waitFile := filepath.Join(root, "opencode-wait")
	serveArgsFile := filepath.Join(root, "opencode-serve-args.txt")
	eventFile := filepath.Join(root, "opencode-events.txt")
	eventData := "event: status\ndata: ready\n\n"
	if err := os.WriteFile(eventFile, []byte(eventData), 0o644); err != nil {
		t.Fatalf("write event data: %v", err)
	}
	opencodePath := filepath.Join(binDir, "opencode")
	opencodeScript := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "serve" ]; then
  shift
  echo "serve $@" > "%s"
  port=""
  for arg in "$@"; do
    case "$arg" in
      --port=*)
        port="${arg#--port=}"
        ;;
    esac
  done
  exec python3 - "$port" "%s" <<'PY'
import sys
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

port = int(sys.argv[1])
event_path = sys.argv[2]

class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass
    def do_GET(self):
        if self.path != "/event":
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        with open(event_path, "rb") as handle:
            self.wfile.write(handle.read())
        self.wfile.flush()
        while True:
            time.sleep(0.1)

server = HTTPServer(("localhost", port), Handler)
server.serve_forever()
PY
fi
if [ "$1" = "run" ]; then
  while [ ! -f "%s" ]; do
    sleep 0.02
  done
  exit 0
fi
exit 0
`, serveArgsFile, eventFile, waitFile)
	if err := os.WriteFile(opencodePath, []byte(opencodeScript), 0o755); err != nil {
		t.Fatalf("write opencode stub: %v", err)
	}

	pathEnv := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, pathEnv))

	projectID := "proj_123"
	sessionID := "ses_123"
	storageRoot := filepath.Join(home, ".local", "share", "opencode", "storage")
	projectDir := filepath.Join(storageRoot, "project")
	sessionDir := filepath.Join(storageRoot, "session", projectID)
	messageDir := filepath.Join(storageRoot, "message", sessionID)
	partDir := filepath.Join(storageRoot, "part", "msg_"+sessionID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}
	projectFile := filepath.Join(projectDir, projectID+".json")
	projectJSON := fmt.Sprintf("{\"id\":\"%s\",\"worktree\":\"%s\"}", projectID, repoPath)
	if err := os.WriteFile(projectFile, []byte(projectJSON), 0o644); err != nil {
		t.Fatalf("write project file: %v", err)
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("create session dir: %v", err)
	}
	if err := os.MkdirAll(messageDir, 0o755); err != nil {
		t.Fatalf("create message dir: %v", err)
	}
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatalf("create part dir: %v", err)
	}

	startedAt := time.Now()
	created := startedAt.Add(1 * time.Second).UnixMilli()
	sessionJSON := fmt.Sprintf("{\"id\":\"%s\",\"projectID\":\"%s\",\"directory\":\"%s\",\"time\":{\"created\":%d}}", sessionID, projectID, repoPath, created)
	if err := os.WriteFile(filepath.Join(sessionDir, sessionID+".json"), []byte(sessionJSON), 0o644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	messageJSON := fmt.Sprintf("{\"id\":\"msg_%s\",\"sessionID\":\"%s\",\"role\":\"user\",\"time\":{\"created\":%d}}", sessionID, sessionID, created)
	if err := os.WriteFile(filepath.Join(messageDir, "msg_"+sessionID+".json"), []byte(messageJSON), 0o644); err != nil {
		t.Fatalf("write message file: %v", err)
	}
	partJSON := fmt.Sprintf("{\"id\":\"prt_%s\",\"sessionID\":\"%s\",\"messageID\":\"msg_%s\",\"type\":\"text\",\"text\":\"Test prompt\"}", sessionID, sessionID, sessionID)
	if err := os.WriteFile(filepath.Join(partDir, "prt_"+sessionID+".json"), []byte(partJSON), 0o644); err != nil {
		t.Fatalf("write part file: %v", err)
	}

	store, err := opencode.Open()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	resultCh := make(chan struct {
		result OpencodeRunResult
		err    error
	}, 1)
	go func() {
		result, err := runOpencodeSession(store, opencodeRunOptions{
			RepoPath:      repoPath,
			WorkspacePath: repoPath,
			Prompt:        "Test prompt",
			StartedAt:     startedAt,
		})
		resultCh <- struct {
			result OpencodeRunResult
			err    error
		}{result: result, err: err}
	}()

	waitForActiveSession(t, store, repoPath, sessionID, 2*time.Second)

	if err := os.WriteFile(waitFile, []byte("done"), 0o644); err != nil {
		t.Fatalf("release opencode: %v", err)
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("run opencode session: %v", result.err)
	}
	if result.result.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, result.result.SessionID)
	}
	if result.result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.result.ExitCode)
	}

	sessions, err := store.ListSessions(repoPath)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Status != opencode.OpencodeSessionCompleted {
		t.Fatalf("expected status completed, got %q", sessions[0].Status)
	}
}

func waitForActiveSession(t *testing.T, store *opencode.Store, repoPath, sessionID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		sessions, err := store.ListSessions(repoPath)
		if err != nil {
			t.Fatalf("list sessions: %v", err)
		}
		for _, session := range sessions {
			if session.ID == sessionID && session.Status == opencode.OpencodeSessionActive {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected active session %s", sessionID)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
