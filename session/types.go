package session

import "github.com/amonks/incrementum/internal/sessionmodel"

// Status represents the session lifecycle state.
type Status = sessionmodel.SessionStatus

const (
	// StatusActive indicates the session is in progress.
	StatusActive Status = sessionmodel.SessionActive
	// StatusCompleted indicates the session completed successfully.
	StatusCompleted Status = sessionmodel.SessionCompleted
	// StatusFailed indicates the session failed.
	StatusFailed Status = sessionmodel.SessionFailed
)

// ValidStatuses returns all valid session status values.
func ValidStatuses() []Status {
	return sessionmodel.ValidSessionStatuses()
}

// Session captures session metadata for a todo.
type Session = sessionmodel.Session
