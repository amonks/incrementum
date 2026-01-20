package session_test

import (
	"encoding/json"
	"fmt"
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
	workspacePath := strings.TrimSpace(output)
	if workspacePath == "" {
		t.Fatalf("expected workspace path output, got: %q", output)
	}
	if _, err := os.Stat(workspacePath); err != nil {
		t.Fatalf("expected workspace path to exist: %v", err)
	}
	expectedWorkspace := filepath.Base(workspacePath)

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
	if sessions[0].WorkspaceName != expectedWorkspace {
		t.Fatalf("expected workspace %s, got %s", expectedWorkspace, sessions[0].WorkspaceName)
	}

	output = runIncr(t, binPath, repoPath, "session", "done", id)
	if !strings.Contains(output, "marked completed") {
		t.Fatalf("expected done output, got: %s", output)
	}
}

func TestE2E_SessionRunOutput(t *testing.T) {
	binPath := buildIncr(t)
	repoPath := setupE2ERepo(t)

	runIncr(t, binPath, repoPath, "todo", "create", "Run todo")

	output := runIncr(t, binPath, repoPath, "todo", "list", "--json")
	var todos []todo.Todo
	if err := json.Unmarshal([]byte(output), &todos); err != nil {
		t.Fatalf("parse todo list: %v", err)
	}
	if len(todos) == 0 {
		t.Fatal("expected todo in list")
	}
	id := todos[0].ID

	output = runIncr(t, binPath, repoPath, "session", "run", id, "--", "true")
	if !strings.Contains(output, "marked completed") {
		t.Fatalf("expected run output, got: %s", output)
	}
}

func TestE2E_SessionStartRevFlag(t *testing.T) {
	binPath := buildIncr(t)
	repoPath := setupE2ERepo(t)

	createJJHistory(t, repoPath, 6)
	revID := strings.TrimSpace(runJJ(t, repoPath, "log", "-r", "@---", "-T", "change_id", "--no-graph"))

	runIncr(t, binPath, repoPath, "todo", "create", "Session rev todo")

	output := runIncr(t, binPath, repoPath, "todo", "list", "--json")
	var todos []todo.Todo
	if err := json.Unmarshal([]byte(output), &todos); err != nil {
		t.Fatalf("parse todo list: %v", err)
	}
	if len(todos) == 0 {
		t.Fatal("expected todo in list")
	}
	id := todos[0].ID

	shellCmd := fmt.Sprintf("cd \"$(\"%s\" session start --rev %s %s)\" && jj log -r @ -T change_id --no-graph", binPath, revID, id)
	cmd := exec.Command("sh", "-c", shellCmd)
	cmd.Dir = repoPath
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("session start --rev failed: %v\noutput: %s", err, outputBytes)
	}
	workspaceChangeID := strings.TrimSpace(string(outputBytes))
	if workspaceChangeID != revID {
		t.Fatalf("expected workspace at %s, got %s", revID, workspaceChangeID)
	}

	runIncr(t, binPath, repoPath, "session", "done", id)
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

func runJJ(t *testing.T, repoPath string, args ...string) string {
	t.Helper()

	cmd := exec.Command("jj", args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj %v failed: %v\noutput: %s", args, err, output)
	}

	return string(output)
}

func createJJHistory(t *testing.T, repoPath string, count int) {
	t.Helper()
	if count < 1 {
		return
	}

	for i := 0; i < count; i++ {
		runJJ(t, repoPath, "describe", "-m", fmt.Sprintf("change %d", i))
		if i < count-1 {
			runJJ(t, repoPath, "new")
		}
	}
}
