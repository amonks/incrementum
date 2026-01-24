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
	prompt := "Test prompt"
	type runOutcome struct {
		result OpencodeRunResult
		err    error
	}
	resultCh := make(chan runOutcome, 1)
	go func() {
		result, err := runOpencodeSession(store, opencodeRunOptions{
			RepoPath:      repoPath,
			WorkspacePath: repoPath,
			Prompt:        prompt,
			StartedAt:     startedAt,
		})
		resultCh <- runOutcome{result: result, err: err}
	}()

	deadline := time.Now().Add(5 * time.Second)
	var result OpencodeRunResult
	resultReady := false
	observedSession := opencode.OpencodeSession{}
	observedActive := false
	for !observedActive && !resultReady {
		if time.Now().After(deadline) {
			t.Fatalf("expected opencode session to reach active state before completion")
		}
		select {
		case outcome := <-resultCh:
			if outcome.err != nil {
				t.Fatalf("run opencode session: %v", outcome.err)
			}
			result = outcome.result
			resultReady = true
		default:
		}

		sessions, err := store.ListSessions(repoPath)
		if err != nil {
			t.Fatalf("list sessions: %v", err)
		}
		for i := range sessions {
			session := sessions[i]
			if !resultReady && session.Prompt == prompt && session.Status == opencode.OpencodeSessionActive {
				observedSession = session
				observedActive = true
				break
			}
		}
		if !observedActive && !resultReady {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if !observedActive {
		t.Fatalf("expected opencode session to reach active state before completion")
	}

	if !resultReady {
		outcome := <-resultCh
		if outcome.err != nil {
			t.Fatalf("run opencode session: %v", outcome.err)
		}
		result = outcome.result
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if observedSession.ID != result.SessionID {
		t.Fatalf("expected session %q, got %q", result.SessionID, observedSession.ID)
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
