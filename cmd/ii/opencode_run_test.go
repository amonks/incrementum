package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/workspace"
)

func TestOpencodeRunLogPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pool, err := workspace.OpenWithOptions(workspace.Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	prompt := "Run tests"
	startedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	sessionID, logPath, err := opencodeRunLogPath(pool, repoPath, prompt, startedAt)
	if err != nil {
		t.Fatalf("get run log path: %v", err)
	}

	expectedID := workspace.GenerateOpencodeSessionID(prompt, startedAt)
	expected := filepath.Join(home, ".local", "share", "incrementum", "opencode", "tmp-my-repo", expectedID+".log")
	if sessionID != expectedID {
		t.Fatalf("expected session id %q, got %q", expectedID, sessionID)
	}
	if logPath != expected {
		t.Fatalf("expected log path %q, got %q", expected, logPath)
	}
}

func TestOpencodeRunAcceptsAttachFalse(t *testing.T) {
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
	opencodePath := filepath.Join(binDir, "opencode")
	opencodeScript := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"run\" ]; then\n  echo \"$@\" > \"%s\"\n  exit 0\nfi\nexit 0\n", argsFile)
	if err := os.WriteFile(opencodePath, []byte(opencodeScript), 0o755); err != nil {
		t.Fatalf("write opencode stub: %v", err)
	}

	jjPath := filepath.Join(binDir, "jj")
	jjScript := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"workspace\" ] && [ \"$2\" = \"root\" ]; then\n  echo \"%s\"\n  exit 0\nfi\necho \"unexpected args\" >&2\nexit 1\n", repoPath)
	if err := os.WriteFile(jjPath, []byte(jjScript), 0o755); err != nil {
		t.Fatalf("write jj stub: %v", err)
	}

	pathEnv := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, pathEnv))

	pool, err := workspace.Open()
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

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

	repoPathForState, err := getOpencodeRepoPath()
	if err != nil {
		t.Fatalf("get repo path: %v", err)
	}
	if _, err := pool.RecordOpencodeDaemon(repoPathForState, os.Getpid(), "", 0, filepath.Join(root, "daemon.log"), time.Now()); err != nil {
		t.Fatalf("record daemon: %v", err)
	}

	if err := opencodeRunCmd.Flags().Set("attach", "false"); err != nil {
		t.Fatalf("set attach flag: %v", err)
	}
	opencodeRunAttach = false
	t.Cleanup(func() {
		_ = opencodeRunCmd.Flags().Set("attach", "true")
		opencodeRunAttach = true
	})

	if err := runOpencodeRun(opencodeRunCmd, []string{"Test prompt"}); err != nil {
		t.Fatalf("run opencode: %v", err)
	}

	var args string
	for i := 0; i < 50; i++ {
		data, err := os.ReadFile(argsFile)
		if err == nil {
			args = strings.TrimSpace(string(data))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if args == "" {
		t.Fatalf("expected opencode args to be recorded")
	}
	if !strings.Contains(args, "--attach") {
		t.Fatalf("expected opencode run to include --attach, got %q", args)
	}
	expectedURL := fmt.Sprintf("http://127.0.0.1:%d", workspace.DefaultOpencodePort)
	if !strings.Contains(args, expectedURL) {
		t.Fatalf("expected opencode run to include attach URL %q, got %q", expectedURL, args)
	}
	if !strings.Contains(args, "Test prompt") {
		t.Fatalf("expected opencode run to include prompt, got %q", args)
	}
}

func TestOpencodeRunUsesWorkingDirectory(t *testing.T) {
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
	opencodePath := filepath.Join(binDir, "opencode")
	opencodeScript := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"run\" ]; then\n  echo \"$@\" > \"%s\"\n  exit 0\nfi\nexit 0\n", argsFile)
	if err := os.WriteFile(opencodePath, []byte(opencodeScript), 0o755); err != nil {
		t.Fatalf("write opencode stub: %v", err)
	}

	t.Setenv("PATH", binDir)

	pool, err := workspace.Open()
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

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

	repoPathForState, err := getOpencodeRepoPath()
	if err != nil {
		t.Fatalf("get repo path: %v", err)
	}
	if _, err := pool.RecordOpencodeDaemon(repoPathForState, os.Getpid(), "", 0, filepath.Join(root, "daemon.log"), time.Now()); err != nil {
		t.Fatalf("record daemon: %v", err)
	}

	if err := opencodeRunCmd.Flags().Set("attach", "true"); err != nil {
		t.Fatalf("set attach flag: %v", err)
	}
	opencodeRunAttach = true

	if err := runOpencodeRun(opencodeRunCmd, []string{"Test prompt"}); err != nil {
		t.Fatalf("run opencode: %v", err)
	}

	var args string
	for i := 0; i < 50; i++ {
		data, err := os.ReadFile(argsFile)
		if err == nil {
			args = strings.TrimSpace(string(data))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if args == "" {
		t.Fatalf("expected opencode args to be recorded")
	}
	if !strings.Contains(args, "--attach") {
		t.Fatalf("expected opencode run to include --attach, got %q", args)
	}
	expectedURL := fmt.Sprintf("http://127.0.0.1:%d", workspace.DefaultOpencodePort)
	if !strings.Contains(args, expectedURL) {
		t.Fatalf("expected opencode run to include attach URL %q, got %q", expectedURL, args)
	}
	if !strings.Contains(args, "Test prompt") {
		t.Fatalf("expected opencode run to include prompt, got %q", args)
	}
}
