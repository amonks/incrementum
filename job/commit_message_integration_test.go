package job

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/internal/jj"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
)

func TestRunCommitMessageShowsSummaryInJjLog(t *testing.T) {
	repoPath := setupJobRepo(t)

	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: true, PromptToCreate: false})
	if err != nil {
		t.Fatalf("open todo store: %v", err)
	}
	created, err := store.Create("Commit summary log", todo.CreateOptions{Priority: todo.PriorityPtr(todo.PriorityLow)})
	if err != nil {
		store.Release()
		t.Fatalf("create todo: %v", err)
	}
	store.Release()

	opencodeCalls := 0
	opts := RunOptions{
		Now: func() time.Time {
			return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		},
		RunTests: func(string, []string) ([]TestCommandResult, error) {
			return []TestCommandResult{{Command: "noop", ExitCode: 0}}, nil
		},
		RunOpencode: func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			opencodeCalls++
			if opencodeCalls == 1 {
				changePath := filepath.Join(runOpts.WorkspacePath, "summary.txt")
				if err := os.WriteFile(changePath, []byte("log summary\n"), 0o644); err != nil {
					return OpencodeRunResult{}, err
				}
				client := jj.New()
				if err := client.Snapshot(runOpts.WorkspacePath); err != nil {
					return OpencodeRunResult{}, err
				}
				messagePath := filepath.Join(runOpts.WorkspacePath, commitMessageFilename)
				message := "\n\nfeat: commit summary    \n\nBody line\n"
				if err := os.WriteFile(messagePath, []byte(message), 0o644); err != nil {
					return OpencodeRunResult{}, err
				}
				return OpencodeRunResult{SessionID: "oc-commit", ExitCode: 0}, nil
			}
			return OpencodeRunResult{SessionID: fmt.Sprintf("oc-%d", opencodeCalls), ExitCode: 0}, nil
		},
	}

	_, err = Run(repoPath, created.ID, opts)
	if err != nil {
		t.Fatalf("run job: %v", err)
	}

	logOutput, err := jjLogDescription(repoPath)
	if err != nil {
		t.Fatalf("jj log: %v", err)
	}
	lines := strings.Split(logOutput, "\n")
	if len(lines) == 0 {
		t.Fatal("expected jj log output")
	}
	if lines[0] != "feat: commit summary" {
		t.Fatalf("expected summary line, got %q", lines[0])
	}
	if internalstrings.TrimTrailingWhitespace(lines[0]) != lines[0] {
		t.Fatalf("expected summary to have no trailing whitespace, got %q", lines[0])
	}
}

func jjLogDescription(repoPath string) (string, error) {
	cmd := exec.Command("jj", "log", "-r", "@-", "--no-graph", "-T", "description")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("jj log output: %w: %s", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("jj log output: %w", err)
	}
	return internalstrings.TrimTrailingNewlines(string(output)), nil
}
