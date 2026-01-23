package main

import (
	"errors"
	"testing"

	sessionpkg "github.com/amonks/incrementum/session"
	"github.com/amonks/incrementum/todo"
)

func TestSessionMutatingOpenOptions(t *testing.T) {
	opts := sessionMutatingOpenOptions(sessionStartCmd, []string{"todo123"})
	if !opts.Todo.CreateIfMissing {
		t.Fatal("expected session mutations to create missing todo store")
	}
	if !opts.Todo.PromptToCreate {
		t.Fatal("expected session mutations to prompt for todo store creation")
	}
	if opts.AllowMissingTodo {
		t.Fatal("expected session mutations to require todo store")
	}
	if opts.Todo.Purpose == "" {
		t.Fatal("expected session mutations to set todo store purpose")
	}
}

func TestRunSessionStartUsesMutatingOpenOptions(t *testing.T) {
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

	err := runSessionStart(sessionStartCmd, []string{"todo123"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	if !got.Todo.CreateIfMissing {
		t.Fatal("expected session start to create missing todo store")
	}
	if !got.Todo.PromptToCreate {
		t.Fatal("expected session start to prompt for todo store creation")
	}
	if got.AllowMissingTodo {
		t.Fatal("expected session start to require todo store")
	}
	if got.Todo.Purpose == "" {
		t.Fatal("expected session start to set todo store purpose")
	}
}

func TestRunSessionFinalizeUsesMutatingOpenOptions(t *testing.T) {
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

	err := runSessionFinalize(sessionDoneCmd, []string{"todo123"}, todo.StatusDone, sessionpkg.StatusCompleted)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	if !got.Todo.CreateIfMissing {
		t.Fatal("expected session finalize to create missing todo store")
	}
	if !got.Todo.PromptToCreate {
		t.Fatal("expected session finalize to prompt for todo store creation")
	}
	if got.AllowMissingTodo {
		t.Fatal("expected session finalize to require todo store")
	}
	if got.Todo.Purpose == "" {
		t.Fatal("expected session finalize to set todo store purpose")
	}
}
