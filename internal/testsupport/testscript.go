package testsupport

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/amonks/incrementum/todo"
	"github.com/rogpeppe/go-internal/testscript"
)

var (
	buildOnce sync.Once
	incrPath  string
	buildErr  error
)

// BuildIncr builds the incr binary once and returns its path.
func BuildIncr(t testing.TB) string {
	t.Helper()

	buildOnce.Do(func() {
		moduleRoot, err := findModuleRoot()
		if err != nil {
			buildErr = err
			return
		}

		binDir, err := os.MkdirTemp("", "incr-bin-")
		if err != nil {
			buildErr = err
			return
		}

		incrPath = filepath.Join(binDir, "incr")
		cmd := exec.Command("go", "build", "-o", incrPath, "./cmd/incr")
		cmd.Dir = moduleRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("build incr: %w: %s", err, strings.TrimSpace(string(output)))
		}
	})

	if buildErr != nil {
		t.Fatalf("%v", buildErr)
	}

	return incrPath
}

// SetupScriptEnv configures common environment variables for testscript.
func SetupScriptEnv(t testing.TB, env *testscript.Env) error {
	t.Helper()

	env.Setenv("INCR", BuildIncr(t))

	homeDir := filepath.Join(env.WorkDir, "home")
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incr"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incr", "workspaces"), 0o755); err != nil {
		return err
	}
	env.Setenv("HOME", homeDir)
	return nil
}

// CmdEnvSet stores the trimmed contents of a file in an env var.
func CmdEnvSet(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("envset does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: envset VAR FILE")
	}

	value := strings.TrimSpace(ts.ReadFile(args[1]))
	ts.Setenv(args[0], value)
}

// CmdTodoID finds a todo by title and stores its ID in an env var.
func CmdTodoID(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("todoid does not support negation")
	}
	if len(args) != 3 {
		ts.Fatalf("usage: todoid FILE TITLE VAR")
	}

	var items []todo.Todo
	data := ts.ReadFile(args[0])
	if err := json.Unmarshal([]byte(data), &items); err != nil {
		ts.Fatalf("parse todo list: %v", err)
	}

	title := args[1]
	for _, item := range items {
		if item.Title == title {
			ts.Setenv(args[2], item.ID)
			return
		}
	}

	ts.Fatalf("todo with title %q not found", title)
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find module root (go.mod)")
		}
		dir = parent
	}
}
