package workspace

import (
	"errors"
	"testing"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestWorkspaceErrorsAliasModel(t *testing.T) {
	if !errors.Is(ErrRepoPathNotFound, statestore.ErrRepoPathNotFound) {
		t.Fatalf("expected ErrRepoPathNotFound to wrap the state error")
	}
}
