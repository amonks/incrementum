package session

import statestore "github.com/amonks/incrementum/internal/state"

// Status represents the session lifecycle state.
type Status = statestore.SessionStatus

const (
	// StatusActive indicates the session is in progress.
	StatusActive Status = statestore.SessionActive
	// StatusCompleted indicates the session completed successfully.
	StatusCompleted Status = statestore.SessionCompleted
	// StatusFailed indicates the session failed.
	StatusFailed Status = statestore.SessionFailed
)

// ValidStatuses returns all valid session status values.
func ValidStatuses() []Status {
	return statestore.ValidSessionStatuses()
}

// Session captures session metadata for a todo.
type Session = statestore.Session
