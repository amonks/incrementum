package main

import (
	"errors"
	"testing"

	sessionpkg "github.com/amonks/incrementum/session"
)

func TestSessionListOpenOptions(t *testing.T) {
	opts := sessionListOpenOptions()
	if opts.Todo.CreateIfMissing {
		t.Fatal("expected session list to avoid creating todo store")
	}
	if opts.Todo.PromptToCreate {
		t.Fatal("expected session list to avoid prompting for todo store creation")
	}
	if !opts.AllowMissingTodo {
		t.Fatal("expected session list to allow missing todo store")
	}
}

func TestRunSessionListUsesListOpenOptions(t *testing.T) {
	original := sessionOpen
	t.Cleanup(func() {
		sessionOpen = original
	})

	var got sessionpkg.OpenOptions
	sentinel := errors.New("sentinel")
	sessionOpen = func(repoPath string, opts sessionpkg.OpenOptions) (*sessionpkg.Manager, error) {
		got = opts
		return nil, sentinel
	}

	err := runSessionList(nil, nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	if got.Todo.CreateIfMissing {
		t.Fatal("expected session list to avoid creating todo store")
	}
	if got.Todo.PromptToCreate {
		t.Fatal("expected session list to avoid prompting for todo store creation")
	}
	if !got.AllowMissingTodo {
		t.Fatal("expected session list to allow missing todo store")
	}
}
