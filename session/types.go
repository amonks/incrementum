package session

import "github.com/amonks/incrementum/workspace"

// Status represents the session lifecycle state.
type Status = workspace.SessionStatus

const (
	// StatusActive indicates the session is in progress.
	StatusActive Status = workspace.SessionActive
	// StatusCompleted indicates the session completed successfully.
	StatusCompleted Status = workspace.SessionCompleted
	// StatusFailed indicates the session failed.
	StatusFailed Status = workspace.SessionFailed
)

// ValidStatuses returns all valid session status values.
func ValidStatuses() []Status {
	return workspace.ValidSessionStatuses()
}

// Session captures session metadata for a todo.
type Session = workspace.Session
