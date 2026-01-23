package job

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/session"
	"github.com/amonks/incrementum/todo"
)

func TestRunIncludesJobIDInSessionTopic(t *testing.T) {
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

	result, err := Run(repoPath, created.ID, RunOptions{
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
	})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	manager, err := session.Open(repoPath, session.OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false},
	})
	if err != nil {
		t.Fatalf("open session manager: %v", err)
	}
	defer manager.Close()

	sessions, err := manager.List(session.ListFilter{IncludeAll: true})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	var matched *session.Session
	for _, item := range sessions {
		if item.ID == result.Job.SessionID {
			entry := item
			matched = &entry
			break
		}
	}
	if matched == nil {
		t.Fatalf("expected session %q", result.Job.SessionID)
	}
	if !strings.Contains(matched.Topic, result.Job.ID) {
		t.Fatalf("expected topic %q to include job id %q", matched.Topic, result.Job.ID)
	}
}
