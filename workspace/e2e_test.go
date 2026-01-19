package workspace_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2E_AcquireReleaseWorkflow tests the full acquire/release workflow using the CLI binary.
func TestE2E_AcquireReleaseWorkflow(t *testing.T) {
	// Build the binary
	binPath := buildBinary(t)

	// Create a temporary jj repo
	repoPath := createJJRepo(t)

	// Set HOME to use our test directories
	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)

	// Copy state dir structure
	t.Setenv("HOME", homeDir)

	// Acquire a workspace
	wsPath := runIncr(t, binPath, repoPath, "workspace", "acquire")
	wsPath = strings.TrimSpace(wsPath)

	t.Logf("Acquired workspace: %s", wsPath)

	// Verify the workspace exists
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Fatalf("workspace directory was not created: %s", wsPath)
	}

	// Verify it's a jj workspace by checking for .jj directory
	jjDir := filepath.Join(wsPath, ".jj")
	if _, err := os.Stat(jjDir); os.IsNotExist(err) {
		t.Error("workspace does not have .jj directory")
	}

	// List workspaces - should show one acquired
	listOutput := runIncr(t, binPath, repoPath, "workspace", "list")
	if !strings.Contains(listOutput, "acquired") {
		t.Errorf("expected 'acquired' in list output, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "ws-001") {
		t.Errorf("expected 'ws-001' in list output, got: %s", listOutput)
	}

	// Release the workspace
	runIncr(t, binPath, repoPath, "workspace", "release", "ws-001")

	// List again - should show available
	listOutput = runIncr(t, binPath, repoPath, "workspace", "list")
	if !strings.Contains(listOutput, "available") {
		t.Errorf("expected 'available' in list output, got: %s", listOutput)
	}

	// Acquire again - should reuse the same workspace
	wsPath2 := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire"))

	if wsPath != wsPath2 {
		t.Errorf("expected workspace to be reused, got %q and %q", wsPath, wsPath2)
	}
}

// TestE2E_MultipleWorkspaces tests acquiring multiple workspaces concurrently.
func TestE2E_MultipleWorkspaces(t *testing.T) {
	binPath := buildBinary(t)
	repoPath := createJJRepo(t)

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	// Acquire two workspaces without releasing
	wsPath1 := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire"))
	wsPath2 := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire"))

	if wsPath1 == wsPath2 {
		t.Error("expected different workspaces")
	}

	// Both should exist
	if _, err := os.Stat(wsPath1); os.IsNotExist(err) {
		t.Errorf("workspace 1 does not exist: %s", wsPath1)
	}
	if _, err := os.Stat(wsPath2); os.IsNotExist(err) {
		t.Errorf("workspace 2 does not exist: %s", wsPath2)
	}

	// List should show two acquired workspaces
	listOutput := runIncr(t, binPath, repoPath, "workspace", "list")
	if strings.Count(listOutput, "acquired") != 2 {
		t.Errorf("expected 2 acquired workspaces in list, got: %s", listOutput)
	}
}

// TestE2E_ListJSON tests JSON output format.
func TestE2E_ListJSON(t *testing.T) {
	binPath := buildBinary(t)
	repoPath := createJJRepo(t)

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	// Acquire a workspace
	runIncr(t, binPath, repoPath, "workspace", "acquire")

	// List with JSON flag
	jsonOutput := runIncr(t, binPath, repoPath, "workspace", "list", "--json")

	// Parse JSON
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &items); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, jsonOutput)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item in JSON, got %d", len(items))
	}

	if items[0]["Name"] != "ws-001" {
		t.Errorf("expected Name 'ws-001', got %v", items[0]["Name"])
	}

	if items[0]["Status"] != "acquired" {
		t.Errorf("expected Status 'acquired', got %v", items[0]["Status"])
	}
}

// TestE2E_Renew tests the renew command.
func TestE2E_Renew(t *testing.T) {
	binPath := buildBinary(t)
	repoPath := createJJRepo(t)

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	// Acquire a workspace
	runIncr(t, binPath, repoPath, "workspace", "acquire")

	// Renew should succeed
	runIncr(t, binPath, repoPath, "workspace", "renew", "ws-001")
}

// TestE2E_ConfigHooks tests on-create and on-acquire hooks.
func TestE2E_ConfigHooks(t *testing.T) {
	binPath := buildBinary(t)
	repoPath := createJJRepo(t)

	// Create a config file with hooks
	configContent := `
[workspace]
on-create = "touch .created"
on-acquire = "touch .acquired"
`
	if err := os.WriteFile(filepath.Join(repoPath, ".incr.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	// Acquire a workspace
	wsPath := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire"))

	// Both hooks should have run
	if _, err := os.Stat(filepath.Join(wsPath, ".created")); os.IsNotExist(err) {
		t.Error("on-create hook did not run")
	}
	if _, err := os.Stat(filepath.Join(wsPath, ".acquired")); os.IsNotExist(err) {
		t.Error("on-acquire hook did not run")
	}

	// Release and acquire again
	runIncr(t, binPath, repoPath, "workspace", "release", "ws-001")

	// Remove the .acquired file to verify it gets recreated
	os.Remove(filepath.Join(wsPath, ".acquired"))

	wsPath2 := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire"))

	if wsPath != wsPath2 {
		t.Errorf("expected same workspace, got %q and %q", wsPath, wsPath2)
	}

	// on-acquire should have run again, but not on-create
	if _, err := os.Stat(filepath.Join(wsPath2, ".acquired")); os.IsNotExist(err) {
		t.Error("on-acquire hook did not run on re-acquire")
	}
}

// TestE2E_RevisionFlag tests the --rev flag.
func TestE2E_RevisionFlag(t *testing.T) {
	binPath := buildBinary(t)
	repoPath := createJJRepo(t)

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	// Acquire with --rev @
	wsPath := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire", "--rev", "@"))

	// Verify it's a valid workspace
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Fatalf("workspace not created: %s", wsPath)
	}
}

// TestE2E_DestroyAll tests the destroy-all command.
func TestE2E_DestroyAll(t *testing.T) {
	binPath := buildBinary(t)
	repoPath := createJJRepo(t)

	homeDir := t.TempDir()
	os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0755)
	t.Setenv("HOME", homeDir)

	// Acquire two workspaces
	wsPath1 := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire"))
	wsPath2 := strings.TrimSpace(runIncr(t, binPath, repoPath, "workspace", "acquire"))

	// Verify workspaces exist
	if _, err := os.Stat(wsPath1); os.IsNotExist(err) {
		t.Fatalf("workspace 1 does not exist: %s", wsPath1)
	}
	if _, err := os.Stat(wsPath2); os.IsNotExist(err) {
		t.Fatalf("workspace 2 does not exist: %s", wsPath2)
	}

	// List should show 2 workspaces
	listOutput := runIncr(t, binPath, repoPath, "workspace", "list")
	if !strings.Contains(listOutput, "ws-001") || !strings.Contains(listOutput, "ws-002") {
		t.Errorf("expected ws-001 and ws-002 in list, got: %s", listOutput)
	}

	// Destroy all
	runIncr(t, binPath, repoPath, "workspace", "destroy-all")

	// Verify workspaces are gone
	if _, err := os.Stat(wsPath1); !os.IsNotExist(err) {
		t.Error("workspace 1 should have been deleted")
	}
	if _, err := os.Stat(wsPath2); !os.IsNotExist(err) {
		t.Error("workspace 2 should have been deleted")
	}

	// List should show no workspaces
	listOutput = runIncr(t, binPath, repoPath, "workspace", "list")
	if !strings.Contains(listOutput, "No workspaces found") {
		t.Errorf("expected 'No workspaces found', got: %s", listOutput)
	}
}

// buildBinary builds the incr binary and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "incr")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/incr")
	cmd.Dir = getProjectRoot(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, output)
	}

	return binPath
}

// createJJRepo creates a temporary jj repository.
func createJJRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	cmd := exec.Command("jj", "git", "init")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to init jj repo: %v\n%s", err, output)
	}

	return tmpDir
}

// runIncr runs the incr binary with the given arguments.
func runIncr(t *testing.T, binPath, workDir string, args ...string) string {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("incr %v failed: %v\nstdout: %s\nstderr: %s", args, err, stdout.String(), stderr.String())
	}

	return stdout.String()
}

// getProjectRoot returns the project root directory.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	// Find go.mod file
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}
