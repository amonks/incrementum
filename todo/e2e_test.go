package todo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/todo"
)

// TestE2E_FullWorkflow tests the complete CLI workflow by shelling out to the incr binary.
func TestE2E_FullWorkflow(t *testing.T) {
	// Build the incr binary
	incrBin := buildIncr(t)

	// Create a test jj repo
	repoPath := setupE2ERepo(t)

	// Test: Create a todo
	t.Run("create todo", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "create", "Fix the login bug", "-t", "bug", "-p", "1")
		if !strings.Contains(output, "Created todo") {
			t.Errorf("expected 'Created todo' in output, got: %s", output)
		}
		if !strings.Contains(output, "Fix the login bug") {
			t.Errorf("expected title in output, got: %s", output)
		}
	})

	// Test: List todos
	var todoID string
	t.Run("list todos", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "list", "--json")
		var todos []todo.Todo
		if err := json.Unmarshal([]byte(output), &todos); err != nil {
			t.Fatalf("failed to parse JSON: %v\noutput: %s", err, output)
		}
		if len(todos) != 1 {
			t.Fatalf("expected 1 todo, got %d", len(todos))
		}
		for _, td := range todos {
			if td.Title == "Fix the login bug" {
				todoID = td.ID
				if td.Type != todo.TypeBug {
					t.Errorf("expected type 'bug', got %q", td.Type)
				}
				if td.Priority != 1 {
					t.Errorf("expected priority 1, got %d", td.Priority)
				}
				break
			}
		}
		if todoID == "" {
			t.Fatal("failed to find 'Fix the login bug' todo")
		}
	})

	// Test: Show todo
	t.Run("show todo", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "show", todoID)
		if !strings.Contains(output, "Fix the login bug") {
			t.Errorf("expected title in output, got: %s", output)
		}
		if !strings.Contains(output, "bug") {
			t.Errorf("expected type 'bug' in output, got: %s", output)
		}
	})

	// Test: Update todo
	t.Run("update todo", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "update", todoID, "--status", "in_progress")
		if !strings.Contains(output, "Updated") {
			t.Errorf("expected 'Updated' in output, got: %s", output)
		}

		// Verify update
		output = runIncr(t, incrBin, repoPath, "todo", "list", "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)
		if todos[0].Status != todo.StatusInProgress {
			t.Errorf("expected status 'in_progress', got %q", todos[0].Status)
		}
	})

	// Test: Create second todo with dependency
	var childID string
	t.Run("create with dependency", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "create", "Write tests", "--deps", "discovered-from:"+todoID)
		if !strings.Contains(output, "Created todo") {
			t.Errorf("expected 'Created todo' in output, got: %s", output)
		}

		// Get the new todo ID
		output = runIncr(t, incrBin, repoPath, "todo", "list", "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)
		for _, td := range todos {
			if td.Title == "Write tests" {
				childID = td.ID
				break
			}
		}
		if childID == "" {
			t.Fatal("failed to find child todo")
		}
	})

	// Test: Add blocking dependency
	t.Run("add dependency", func(t *testing.T) {
		// Create a blocker todo
		runIncr(t, incrBin, repoPath, "todo", "create", "Blocker task")

		// Get its ID
		output := runIncr(t, incrBin, repoPath, "todo", "list", "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)

		var blockerID string
		for _, td := range todos {
			if td.Title == "Blocker task" {
				blockerID = td.ID
				break
			}
		}

		// Add blocking dependency
		output = runIncr(t, incrBin, repoPath, "todo", "dep", "add", childID, blockerID, "--type", "blocks")
		if !strings.Contains(output, "Added dependency") {
			t.Errorf("expected 'Added dependency' in output, got: %s", output)
		}
	})

	// Test: Dep tree
	t.Run("dep tree", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "dep", "tree", childID)
		if !strings.Contains(output, "Write tests") {
			t.Errorf("expected 'Write tests' in tree output, got: %s", output)
		}
		// Should show the discovered-from parent
		if !strings.Contains(output, "discovered-from") {
			t.Errorf("expected 'discovered-from' in tree output, got: %s", output)
		}
	})

	// Test: Ready command
	t.Run("ready", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "ready", "--json")
		var readyTodos []todo.Todo
		json.Unmarshal([]byte(output), &readyTodos)

		// "Write tests" should NOT be ready (blocked by "Blocker task")
		for _, td := range readyTodos {
			if td.Title == "Write tests" {
				t.Error("'Write tests' should not be ready - it's blocked")
			}
		}

		// "Blocker task" should be ready
		found := false
		for _, td := range readyTodos {
			if td.Title == "Blocker task" {
				found = true
				break
			}
		}
		if !found {
			t.Error("'Blocker task' should be ready")
		}
	})

	// Test: Close todo
	t.Run("close todo", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "close", todoID, "--reason", "Fixed!")
		if !strings.Contains(output, "Closed") {
			t.Errorf("expected 'Closed' in output, got: %s", output)
		}

		// Verify
		output = runIncr(t, incrBin, repoPath, "todo", "show", todoID, "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)
		if todos[0].Status != todo.StatusClosed {
			t.Errorf("expected status 'closed', got %q", todos[0].Status)
		}
	})

	// Test: Reopen todo
	t.Run("reopen todo", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "reopen", todoID)
		if !strings.Contains(output, "Reopened") {
			t.Errorf("expected 'Reopened' in output, got: %s", output)
		}

		// Verify
		output = runIncr(t, incrBin, repoPath, "todo", "show", todoID, "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)
		if todos[0].Status != todo.StatusOpen {
			t.Errorf("expected status 'open', got %q", todos[0].Status)
		}
	})

	// Test: Filter by status
	t.Run("list filter by status", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "list", "--status", "open", "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)
		for _, td := range todos {
			if td.Status != todo.StatusOpen {
				t.Errorf("expected only open todos, got status %q for %q", td.Status, td.Title)
			}
		}
	})

	// Test: Filter by type
	t.Run("list filter by type", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "list", "--type", "bug", "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)
		if len(todos) != 1 {
			t.Errorf("expected 1 bug, got %d", len(todos))
		}
		if len(todos) > 0 && todos[0].Type != todo.TypeBug {
			t.Errorf("expected type 'bug', got %q", todos[0].Type)
		}
	})

	// Test: Filter by title substring
	t.Run("list filter by title", func(t *testing.T) {
		output := runIncr(t, incrBin, repoPath, "todo", "list", "--title", "login", "--json")
		var todos []todo.Todo
		json.Unmarshal([]byte(output), &todos)
		if len(todos) != 1 {
			t.Errorf("expected 1 todo matching 'login', got %d", len(todos))
		}
	})
}

// TestE2E_CreatePrompt tests that creating the todo store requires confirmation.
func TestE2E_CreatePrompt(t *testing.T) {
	incrBin := buildIncr(t)
	repoPath := setupE2ERepo(t)

	// Run with "n" input - should fail
	cmd := exec.Command(incrBin, "todo", "list")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("n\n")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when declining to create todo store")
	}
	if !strings.Contains(string(output), "No todo store found") || !strings.Contains(string(output), "Create one?") {
		t.Errorf("expected prompt message, got: %s", output)
	}

	// Run with "y" input - should succeed
	cmd = exec.Command(incrBin, "todo", "list")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("y\n")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected success when accepting to create todo store: %v\noutput: %s", err, output)
	}
}

// TestE2E_MultipleIDs tests operations on multiple todos at once.
func TestE2E_DescriptionFromStdin(t *testing.T) {
	incrBin := buildIncr(t)
	repoPath := setupE2ERepo(t)

	runIncr(t, incrBin, repoPath, "todo", "create", "Seed todo")

	cmd := exec.Command(incrBin, "todo", "create", "Stdin todo", "--desc", "-")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("Created from stdin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("todo create with stdin failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(string(output), "Created todo") {
		t.Fatalf("expected create output, got: %s", output)
	}

	listOutput := runIncr(t, incrBin, repoPath, "todo", "list", "--json")
	var todos []todo.Todo
	if err := json.Unmarshal([]byte(listOutput), &todos); err != nil {
		t.Fatalf("failed to parse list JSON: %v\noutput: %s", err, listOutput)
	}

	var stdinID string
	for _, td := range todos {
		if td.Title == "Stdin todo" {
			stdinID = td.ID
			if td.Description != "Created from stdin" {
				t.Fatalf("expected description from stdin, got %q", td.Description)
			}
		}
	}
	if stdinID == "" {
		t.Fatal("failed to find stdin todo")
	}

	cmd = exec.Command(incrBin, "todo", "update", stdinID, "--desc", "-")
	cmd.Dir = repoPath
	cmd.Stdin = strings.NewReader("Updated from stdin")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("todo update with stdin failed: %v\noutput: %s", err, output)
	}
	if !strings.Contains(string(output), "Updated") {
		t.Fatalf("expected update output, got: %s", output)
	}

	showOutput := runIncr(t, incrBin, repoPath, "todo", "show", stdinID, "--json")
	var showTodos []todo.Todo
	if err := json.Unmarshal([]byte(showOutput), &showTodos); err != nil {
		t.Fatalf("failed to parse show JSON: %v\noutput: %s", err, showOutput)
	}
	if len(showTodos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(showTodos))
	}
	if showTodos[0].Description != "Updated from stdin" {
		t.Fatalf("expected updated description, got %q", showTodos[0].Description)
	}
}

func TestE2E_MultipleIDs(t *testing.T) {
	incrBin := buildIncr(t)
	repoPath := setupE2ERepo(t)

	// Create multiple todos
	runIncr(t, incrBin, repoPath, "todo", "create", "Task 1")
	runIncr(t, incrBin, repoPath, "todo", "create", "Task 2")
	runIncr(t, incrBin, repoPath, "todo", "create", "Task 3")

	// Get IDs
	output := runIncr(t, incrBin, repoPath, "todo", "list", "--json")
	var todos []todo.Todo
	json.Unmarshal([]byte(output), &todos)

	ids := make([]string, len(todos))
	for i, td := range todos {
		ids[i] = td.ID
	}

	// Close multiple
	args := append([]string{"todo", "close"}, ids...)
	output = runIncr(t, incrBin, repoPath, args...)
	for _, td := range todos {
		if !strings.Contains(output, td.ID) {
			t.Errorf("expected ID %q in output", td.ID)
		}
	}

	// Verify all closed
	output = runIncr(t, incrBin, repoPath, "todo", "list", "--json")
	json.Unmarshal([]byte(output), &todos)
	for _, td := range todos {
		if td.Status != todo.StatusClosed {
			t.Errorf("expected all todos closed, but %q has status %q", td.Title, td.Status)
		}
	}

	// Reopen multiple
	args = append([]string{"todo", "reopen"}, ids...)
	runIncr(t, incrBin, repoPath, args...)

	// Verify all open
	output = runIncr(t, incrBin, repoPath, "todo", "list", "--json")
	json.Unmarshal([]byte(output), &todos)
	for _, td := range todos {
		if td.Status != todo.StatusOpen {
			t.Errorf("expected all todos open, but %q has status %q", td.Title, td.Status)
		}
	}

	// Update multiple
	args = append([]string{"todo", "update"}, ids...)
	args = append(args, "--status", "in_progress")
	output = runIncr(t, incrBin, repoPath, args...)
	for _, td := range todos {
		if !strings.Contains(output, td.ID) {
			t.Errorf("expected ID %q in update output", td.ID)
		}
	}

	// Verify all in progress
	output = runIncr(t, incrBin, repoPath, "todo", "list", "--json")
	json.Unmarshal([]byte(output), &todos)
	for _, td := range todos {
		if td.Status != todo.StatusInProgress {
			t.Errorf("expected all todos in progress, but %q has status %q", td.Title, td.Status)
		}
	}
}

// TestE2E_ReadyWithBlockers tests that the ready command properly filters blocked todos.
func TestE2E_ReadyWithBlockers(t *testing.T) {
	incrBin := buildIncr(t)
	repoPath := setupE2ERepo(t)

	// Create blocker and blocked todos
	runIncr(t, incrBin, repoPath, "todo", "create", "Blocker", "-p", "0")
	runIncr(t, incrBin, repoPath, "todo", "create", "Blocked", "-p", "0")

	// Get IDs
	output := runIncr(t, incrBin, repoPath, "todo", "list", "--json")
	var todos []todo.Todo
	json.Unmarshal([]byte(output), &todos)

	var blockerID, blockedID string
	for _, td := range todos {
		if td.Title == "Blocker" {
			blockerID = td.ID
		} else if td.Title == "Blocked" {
			blockedID = td.ID
		}
	}

	// Add blocking dependency
	runIncr(t, incrBin, repoPath, "todo", "dep", "add", blockedID, blockerID, "--type", "blocks")

	// Check ready - only Blocker should be ready
	output = runIncr(t, incrBin, repoPath, "todo", "ready", "--json")
	var ready []todo.Todo
	json.Unmarshal([]byte(output), &ready)

	if len(ready) != 1 {
		t.Fatalf("expected 1 ready todo, got %d", len(ready))
	}
	if ready[0].Title != "Blocker" {
		t.Errorf("expected 'Blocker' to be ready, got %q", ready[0].Title)
	}

	// Close blocker
	runIncr(t, incrBin, repoPath, "todo", "close", blockerID)

	// Now Blocked should be ready
	output = runIncr(t, incrBin, repoPath, "todo", "ready", "--json")
	json.Unmarshal([]byte(output), &ready)

	if len(ready) != 1 {
		t.Fatalf("expected 1 ready todo after closing blocker, got %d", len(ready))
	}
	if ready[0].Title != "Blocked" {
		t.Errorf("expected 'Blocked' to be ready, got %q", ready[0].Title)
	}
}

// buildIncr builds the incr binary and returns its path.
func buildIncr(t *testing.T) string {
	t.Helper()

	moduleRoot := findModuleRoot(t)

	// Build to a temp location
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

// setupE2ERepo creates a temporary jj repository for e2e testing.
// It also sets HOME to a temp directory to prevent leaking state into
// ~/.local/state/incr and ~/.local/share/incr/workspaces.
func setupE2ERepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	// Set HOME to a temp directory to isolate state/workspaces
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

// runIncr runs the incr command and returns its output.
func runIncr(t *testing.T, binPath, repoPath string, args ...string) string {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	cmd.Dir = repoPath
	// Provide "y" input for any prompts
	cmd.Stdin = strings.NewReader("y\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("incr %s failed: %v\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), err, stdout.String(), stderr.String())
	}

	return stdout.String()
}
