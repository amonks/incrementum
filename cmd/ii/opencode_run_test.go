package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/opencode"
)

func TestOpencodeRunShellsOutAndRecordsSession(t *testing.T) {
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

	argsFile := filepath.Join(root, "opencode-args.txt")
	projectID := "proj_123"
	sessionID := "ses_123"

	opencodePath := filepath.Join(binDir, "opencode")
	opencodeScript := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"run\" ]; then\n  echo \"$@\" > \"%s\"\n  exit 0\nfi\nexit 0\n", argsFile)
	if err := os.WriteFile(opencodePath, []byte(opencodeScript), 0o755); err != nil {
		t.Fatalf("write opencode stub: %v", err)
	}

	pathEnv := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, pathEnv))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	repoPath, err = os.Getwd()
	if err != nil {
		t.Fatalf("get repo cwd: %v", err)
	}
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

	created := time.Now().Add(10 * time.Second).UnixMilli()
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

	if err := runOpencodeRun(opencodeRunCmd, []string{"Test prompt"}); err != nil {
		t.Fatalf("run opencode: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.TrimSpace(string(data))
	if !strings.Contains(args, "run") {
		t.Fatalf("expected opencode run args, got %q", args)
	}
	if strings.Contains(args, "--attach") {
		t.Fatalf("expected opencode run without --attach, got %q", args)
	}
	if !strings.Contains(args, "Test prompt") {
		t.Fatalf("expected prompt to be passed, got %q", args)
	}

	store, err := opencode.Open()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	sessions, err := store.ListSessions(repoPath)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, sessions[0].ID)
	}
	if sessions[0].Status != opencode.OpencodeSessionCompleted {
		t.Fatalf("expected status completed, got %q", sessions[0].Status)
	}
	if sessions[0].ExitCode == nil || *sessions[0].ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %v", sessions[0].ExitCode)
	}
}
