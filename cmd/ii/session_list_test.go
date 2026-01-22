package main

import "testing"

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
