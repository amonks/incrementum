package session

import (
	"testing"

	"github.com/amonks/incrementum/internal/sessionmodel"
)

func TestStatusAndSessionAliasesModel(t *testing.T) {
	var status Status = sessionmodel.SessionActive
	if status != StatusActive {
		t.Fatalf("expected status alias to match model")
	}

	var item Session = sessionmodel.Session{}
	if item.ID != "" {
		t.Fatalf("expected session alias to match model")
	}
}
