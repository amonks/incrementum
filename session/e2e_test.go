package session_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/session"
	"github.com/amonks/incrementum/todo"
)

func TestE2E_SessionStartDoneList(t *testing.T) {
	binPath := buildIncr(t)
	repoPath := setupE2ERepo(t)

	runIncr(t, binPath, repoPath, "todo", "create", "Session todo")

	output := runIncr(t, binPath, repoPath, "todo", "list", "--json")
	var todos []todo.Todo
	if err := json.Unmarshal([]byte(output), &todos); err != nil {
		t.Fatalf("parse todo list: %v", err)
	}
	if len(todos) == 0 {
		t.Fatal("expected todo in list")
	}
	id := todos[0].ID

	output = runIncr(t, binPath, repoPath, "session", "start", id)
	if !strings.Contains(output, "Started session") {
		t.Fatalf("expected start output, got: %s", output)
	}

	output = runIncr(t, binPath, repoPath, "session", "list", "--json")
	var sessions []session.Session
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		t.Fatalf("parse session list: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].TodoID != id {
		t.Fatalf("expected todo id %s, got %s", id, sessions[0].TodoID)
	}
	if sessions[0].Status != session.StatusActive {
		t.Fatalf("expected active status, got %s", sessions[0].Status)
	}

	output = runIncr(t, binPath, repoPath, "session", "done", id)
	if !strings.Contains(output, "marked completed") {
		t.Fatalf("expected done output, got: %s", output)
	}
}

func buildIncr(t *testing.T) string {
	t.Helper()

	moduleRoot := findModuleRoot(t)

	binPath := filepath.Join(t.TempDir(), "incr")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/incr")
	cmd.Dir = moduleRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build incr: %v\n%s", err, output)
	}

	return binPath
}

func findModuleRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	moduleRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(moduleRoot)
		if parent == moduleRoot {
			t.Fatal("could not find module root (go.mod)")
		}
		moduleRoot = parent
	}

	return moduleRoot
}

func setupE2ERepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	return tmpDir
}

func runIncr(t *testing.T, binPath, repoPath string, args ...string) string {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("y\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("incr %v failed: %v\noutput: %s", args, err, output)
	}

	return string(output)
}
