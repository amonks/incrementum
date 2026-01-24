package opencode

import (
	"errors"

	statestore "github.com/amonks/incrementum/internal/state"
)

// OpencodeSessionStatus represents the state of an opencode session.
type OpencodeSessionStatus = statestore.OpencodeSessionStatus

const (
	// OpencodeSessionActive indicates the session is active.
	OpencodeSessionActive OpencodeSessionStatus = statestore.OpencodeSessionActive
	// OpencodeSessionCompleted indicates the session completed successfully.
	OpencodeSessionCompleted OpencodeSessionStatus = statestore.OpencodeSessionCompleted
	// OpencodeSessionFailed indicates the session failed.
	OpencodeSessionFailed OpencodeSessionStatus = statestore.OpencodeSessionFailed
	// OpencodeSessionKilled indicates the session was terminated.
	OpencodeSessionKilled OpencodeSessionStatus = statestore.OpencodeSessionKilled
)

// OpencodeSession stores session state for a repo.
type OpencodeSession = statestore.OpencodeSession

var (
	// ErrOpencodeSessionNotFound indicates the requested session is missing.
	ErrOpencodeSessionNotFound = errors.New("opencode session not found")
	// ErrAmbiguousOpencodeSessionIDPrefix indicates a prefix matches multiple sessions.
	ErrAmbiguousOpencodeSessionIDPrefix = errors.New("ambiguous opencode session id prefix")
	// ErrOpencodeSessionNotActive indicates a session is not active.
	ErrOpencodeSessionNotActive = errors.New("opencode session is not active")
)
