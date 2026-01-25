package opencode

import (
	"strings"
	"testing"

	"github.com/amonks/incrementum/internal/todoenv"
)

func TestReplaceEnvVarOverridesExisting(t *testing.T) {
	env := []string{"PWD=/old", "PATH=/bin", "PWD=/older"}

	updated := replaceEnvVar(env, "PWD", "/new")

	if !containsEnv(updated, "PATH=/bin") {
		t.Fatalf("expected PATH to be preserved")
	}

	count := 0
	value := ""
	for _, entry := range updated {
		if strings.HasPrefix(entry, "PWD=") {
			count++
			value = strings.TrimPrefix(entry, "PWD=")
		}
	}

	if count != 1 {
		t.Fatalf("expected single PWD entry, got %d", count)
	}
	if value != "/new" {
		t.Fatalf("expected PWD to be %q, got %q", "/new", value)
	}
}

func TestEnsureTodoProposerEnvSetsValue(t *testing.T) {
	env := []string{"PATH=/bin", todoenv.ProposerEnvVar + "=false"}

	updated := ensureTodoProposerEnv(env)

	if !containsEnv(updated, todoenv.ProposerEnvVar+"=true") {
		t.Fatalf("expected proposer env to be true, got %v", updated)
	}
}

func containsEnv(env []string, entry string) bool {
	for _, item := range env {
		if item == entry {
			return true
		}
	}
	return false
}
