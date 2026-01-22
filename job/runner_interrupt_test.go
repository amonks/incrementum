package job

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/session"
	"github.com/amonks/incrementum/todo"
)

func TestRunInterruptMarksJobFailed(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Interrupt job", todo.CreateOptions{Priority: todo.PriorityPtr(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	started := make(chan struct{})
	block := make(chan struct{})
	done := make(chan struct{})

	var result *RunResult
	var runErr error
	go func() {
		result, runErr = Run(repoPath, created.ID, RunOptions{
			Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
			RunOpencode: func(opts opencodeRunOptions) (OpencodeRunResult, error) {
				close(started)
				<-block
				return OpencodeRunResult{SessionID: "opencode-1", ExitCode: 0}, nil
			},
		})
		close(done)
	}()

	<-started

	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		t.Fatalf("send interrupt: %v", err)
	}

	close(block)
	<-done

	if runErr == nil {
		t.Fatal("expected interrupt error")
	}
	if !errors.Is(runErr, ErrJobInterrupted) {
		t.Fatalf("expected interrupt error, got %v", runErr)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Job.Status != StatusFailed {
		t.Fatalf("expected failed status, got %q", result.Job.Status)
	}

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		t.Fatalf("reopen todo store: %v", err)
	}
	items, err := store.Show([]string{created.ID})
	if err != nil {
		store.Release()
		t.Fatalf("show todo: %v", err)
	}
	if items[0].Status != todo.StatusOpen {
		store.Release()
		t.Fatalf("expected todo status open, got %q", items[0].Status)
	}
	store.Release()

	sessionManager, err := session.Open(repoPath, session.OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false},
	})
	if err != nil {
		t.Fatalf("open session manager: %v", err)
	}
	defer sessionManager.Close()

	sessions, err := sessionManager.List(session.ListFilter{})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no active sessions, got %d", len(sessions))
	}
}

func setupJobRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	homeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incrementum"), 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incrementum", "workspaces"), 0o755); err != nil {
		t.Fatalf("create workspace dir: %v", err)
	}
	t.Setenv("HOME", homeDir)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	return tmpDir
}
