package workspace

import (
	"testing"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestSessionTypesAliasModel(t *testing.T) {
	var status SessionStatus = statestore.SessionActive
	if status != SessionActive {
		t.Fatalf("expected session status alias to match model")
	}

	var item Session = statestore.Session{}
	if item.ID != "" {
		t.Fatalf("expected session alias to match model")
	}
}
