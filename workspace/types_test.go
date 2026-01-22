package workspace

import (
	"testing"

	"github.com/amonks/incrementum/internal/sessionmodel"
)

func TestSessionTypesAliasModel(t *testing.T) {
	var status SessionStatus = sessionmodel.SessionActive
	if status != SessionActive {
		t.Fatalf("expected session status alias to match model")
	}

	var item Session = sessionmodel.Session{}
	if item.ID != "" {
		t.Fatalf("expected session alias to match model")
	}
}
