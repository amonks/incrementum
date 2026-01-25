package job

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/todo"
)

func TestRunMarksTodoInProgress(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Job topic", todo.CreateOptions{Priority: todo.PriorityPtr(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	opencodeCount := 0

	_, err = Run(repoPath, created.ID, RunOptions{
		Now: func() time.Time { return now },
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return nil, nil
		},
		UpdateStale: func(string) error { return nil },
		RunOpencode: func(opts opencodeRunOptions) (OpencodeRunResult, error) {
			opencodeCount++
			if opencodeCount == 3 {
				messagePath := filepath.Join(opts.WorkspacePath, commitMessageFilename)
				if err := os.WriteFile(messagePath, []byte("feat: add topic"), 0o644); err != nil {
					return OpencodeRunResult{}, err
				}
			}
			return OpencodeRunResult{SessionID: fmt.Sprintf("opencode-%d", opencodeCount), ExitCode: 0}, nil
		},
		OnStart: func(StartInfo) {
			store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
			if err != nil {
				t.Fatalf("open todo store: %v", err)
			}
			items, err := store.Show([]string{created.ID})
			if err != nil {
				store.Release()
				t.Fatalf("show todo: %v", err)
			}
			status := items[0].Status
			store.Release()
			if status != todo.StatusInProgress {
				t.Fatalf("expected todo in progress, got %q", status)
			}
		},
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	store, err = todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	items, err := store.Show([]string{created.ID})
	if err != nil {
		store.Release()
		t.Fatalf("show todo: %v", err)
	}
	status := items[0].Status
	store.Release()
	if status != todo.StatusDone {
		t.Fatalf("expected todo done, got %q", status)
	}
}

func TestRunStoresOpencodeAgent(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Agent tracking", todo.CreateOptions{Priority: todo.PriorityPtr(todo.PriorityMedium)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	now := time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC)
	opencodeCount := 0

	result, err := Run(repoPath, created.ID, RunOptions{
		Now: func() time.Time { return now },
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{}, nil
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return nil, nil
		},
		UpdateStale: func(string) error { return nil },
		CurrentCommitID: func(string) (string, error) {
			return "same", nil
		},
		RunOpencode: func(opencodeRunOptions) (OpencodeRunResult, error) {
			opencodeCount++
			return OpencodeRunResult{SessionID: fmt.Sprintf("opencode-%d", opencodeCount), ExitCode: 0}, nil
		},
		OpencodeAgent: "agent-42",
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	if result.Job.Agent != "agent-42" {
		t.Fatalf("expected agent on result job, got %q", result.Job.Agent)
	}

	manager, err := Open(repoPath, OpenOptions{})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}
	stored, err := manager.Find(result.Job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	if stored.Agent != "agent-42" {
		t.Fatalf("expected agent in state, got %q", stored.Agent)
	}
}
