package job

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		if currentHome := os.Getenv("HOME"); currentHome != "" {
			configHome = filepath.Join(currentHome, ".config")
		}
	}
	if configHome == "" {
		t.Skip("opencode config home is not set")
	}
	configDir := filepath.Join(configHome, "opencode")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Skipf("opencode config not found at %s: %v", configDir, err)
	}
	if len(entries) == 0 {
		t.Skipf("opencode config directory %s is empty", configDir)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CONFIG_HOME", configHome)

	return home
}

func TestRunOpencodeSessionRecordsSession(t *testing.T) {
	requireOpencode(t)
	prepareOpencodeHome(t)

	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
	}

	store, err := opencode.Open()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	startedAt := time.Now()
	type runOutcome struct {
		result OpencodeRunResult
		err    error
	}
	resultCh := make(chan runOutcome, 1)
	go func() {
		result, err := runOpencodeSession(store, opencodeRunOptions{
			RepoPath:      repoPath,
			WorkspacePath: repoPath,
			Prompt:        "Test prompt",
			StartedAt:     startedAt,
		})
		resultCh <- runOutcome{result: result, err: err}
	}()

	deadline := time.Now().Add(5 * time.Second)
	activeSessionID := ""
	for activeSessionID == "" {
		if time.Now().After(deadline) {
			t.Fatalf("expected active opencode session before completion")
		}
		select {
		case outcome := <-resultCh:
			if outcome.err != nil {
				t.Fatalf("run opencode session: %v", outcome.err)
			}
			t.Fatalf("opencode session %q completed before active status was observed", outcome.result.SessionID)
		default:
			sessions, err := store.ListSessions(repoPath)
			if err != nil {
				t.Fatalf("list sessions: %v", err)
			}
			for i := range sessions {
				if sessions[i].Status == opencode.OpencodeSessionActive {
					activeSessionID = sessions[i].ID
					break
				}
			}
			if activeSessionID == "" {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}

	outcome := <-resultCh
	if outcome.err != nil {
		t.Fatalf("run opencode session: %v", outcome.err)
	}
	result := outcome.result
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if activeSessionID != result.SessionID {
		t.Fatalf("expected active session %q, got %q", result.SessionID, activeSessionID)
	}

	sessions, err := store.ListSessions(repoPath)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatalf("expected at least 1 session, got 0")
	}
	var matched *opencode.OpencodeSession
	for i := range sessions {
		if sessions[i].ID == result.SessionID {
			matched = &sessions[i]
			break
		}
	}
	if matched == nil {
		t.Fatalf("expected session %q, got %+v", result.SessionID, sessions)
	}
	if matched.Status != opencode.OpencodeSessionCompleted {
		t.Fatalf("expected status completed, got %q", matched.Status)
	}
	logs, err := store.Logs(repoPath, matched.ID)
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	if strings.TrimSpace(logs) == "" {
		t.Fatalf("expected opencode event log content")
	}
	if !strings.Contains(logs, "data:") {
		t.Fatalf("expected opencode event log to include data lines")
	}
}
