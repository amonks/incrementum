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

func TestWorkspaceTypesAliasModel(t *testing.T) {
	var status Status = statestore.WorkspaceStatusAvailable
	if status != StatusAvailable {
		t.Fatalf("expected workspace status alias to match model")
	}
}

func TestValidStatusesReturnsModelValues(t *testing.T) {
	statuses := ValidStatuses()
	if len(statuses) != len(statestore.ValidWorkspaceStatuses()) {
		t.Fatalf("expected %d statuses, got %d", len(statestore.ValidWorkspaceStatuses()), len(statuses))
	}
}

func TestWorkspaceStatusIsValid(t *testing.T) {
	if !StatusAvailable.IsValid() {
		t.Fatalf("expected status to be valid")
	}

	if Status("nope").IsValid() {
		t.Fatalf("expected status to be invalid")
	}
}
