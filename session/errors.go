package session

import (
	"errors"

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
