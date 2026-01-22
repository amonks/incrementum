package session

import (
	"testing"

	"github.com/amonks/incrementum/workspace"
)

func TestStatusAndSessionAliasesWorkspace(t *testing.T) {
	var status Status = workspace.SessionActive
	if status != StatusActive {
		t.Fatalf("expected status alias to match workspace")
	}

	var item Session = workspace.Session{}
	if item.ID != "" {
		t.Fatalf("expected session alias to match workspace")
	}
}
