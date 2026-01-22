package session

import (
	"testing"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestStatusAndSessionAliasesModel(t *testing.T) {
	var status Status = statestore.SessionActive
	if status != StatusActive {
		t.Fatalf("expected status alias to match model")
	}

	var item Session = statestore.Session{}
	if item.ID != "" {
		t.Fatalf("expected session alias to match model")
	}
}
