package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/opencode"
)

func requireOpencode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skipf("opencode binary is required for integration tests: %v", err)
	}
}

func prepareOpencodeHome(t *testing.T) string {
	t.Helper()

	realConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if realConfigHome == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("resolve user home: %v", err)
		}
		realConfigHome = filepath.Join(userHome, ".config")
	}
	realConfigPath := filepath.Join(realConfigHome, "opencode", "opencode.json")
	configContents, err := os.ReadFile(realConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("opencode config %s is required for integration tests", realConfigPath)
		}
		t.Fatalf("read config file: %v", err)
	}

	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	configDir := filepath.Join(configHome, "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, configContents, 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CONFIG_HOME", configHome)

	return home
}

func prepareOpencodeRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
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

	return repoPath
}

func TestOpencodeRunShellsOutAndRecordsSession(t *testing.T) {
	requireOpencode(t)
	prepareOpencodeHome(t)
	repoPath := prepareOpencodeRepo(t)

	prompt := "Test prompt"
	if err := runOpencodeRun(opencodeRunCmd, []string{prompt}); err != nil {
		t.Fatalf("run opencode: %v", err)
	}

	store, err := opencode.Open()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	sessions, err := store.ListSessions(repoPath)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatalf("expected at least 1 session, got 0")
	}
	// Get the most recent session (list is sorted by start time)
	matched := &sessions[len(sessions)-1]
	if matched.Status != opencode.OpencodeSessionCompleted {
		t.Fatalf("expected status completed, got %q", matched.Status)
	}
	if matched.ExitCode == nil || *matched.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %v", matched.ExitCode)
	}
	logs, err := store.Logs(repoPath, matched.ID)
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	if internalstrings.IsBlank(logs) {
		t.Fatalf("expected opencode event log content")
	}
	if !strings.Contains(logs, "data:") {
		t.Fatalf("expected opencode event log to include data lines")
	}
}

func TestOpencodeRunUsesStdinPrompt(t *testing.T) {
	requireOpencode(t)
	prepareOpencodeHome(t)
	repoPath := prepareOpencodeRepo(t)

	prompt := "Prompt from stdin"
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	_, err = writer.WriteString(prompt + "\n")
	if err != nil {
		_ = reader.Close()
		_ = writer.Close()
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		_ = reader.Close()
		t.Fatalf("close stdin writer: %v", err)
	}

	originalStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = reader.Close()
	})

	if err := runOpencodeRun(opencodeRunCmd, nil); err != nil {
		t.Fatalf("run opencode: %v", err)
	}

	store, err := opencode.Open()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	sessions, err := store.ListSessions(repoPath)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatalf("expected at least 1 session, got 0")
	}
	// Get the most recent session (list is sorted by start time)
	matched := &sessions[len(sessions)-1]
	if matched.Status != opencode.OpencodeSessionCompleted {
		t.Fatalf("expected status completed, got %q", matched.Status)
	}
}
