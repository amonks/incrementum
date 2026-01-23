package session

import (
	"errors"
	"fmt"

	"github.com/amonks/incrementum/internal/validation"
	"github.com/amonks/incrementum/workspace"
)

var (
	// ErrInvalidStatus indicates a session status is invalid.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrSessionAlreadyActive indicates a todo already has an active session.
	ErrSessionAlreadyActive = workspace.ErrSessionAlreadyActive
	// ErrSessionNotFound indicates the requested session is missing.
	ErrSessionNotFound = workspace.ErrSessionNotFound
	// ErrSessionNotActive indicates a session is not active.
	ErrSessionNotActive = workspace.ErrSessionNotActive
)

func formatInvalidStatusError(status Status) error {
	return fmt.Errorf("%w: %q (valid: %s)", ErrInvalidStatus, status, validation.FormatValidValues(ValidStatuses()))
}
