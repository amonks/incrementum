package sessionmodel

import "time"

// SessionStatus represents the state of a session.
type SessionStatus string

const (
	// SessionActive indicates the session is still active.
	SessionActive SessionStatus = "active"
	// SessionCompleted indicates the session completed successfully.
	SessionCompleted SessionStatus = "completed"
	// SessionFailed indicates the session failed.
	SessionFailed SessionStatus = "failed"
)

// ValidSessionStatuses returns all valid session status values.
func ValidSessionStatuses() []SessionStatus {
	return []SessionStatus{SessionActive, SessionCompleted, SessionFailed}
}

// IsValid returns true if the status is a known value.
func (s SessionStatus) IsValid() bool {
	for _, valid := range ValidSessionStatuses() {
		if s == valid {
			return true
		}
	}
	return false
}

// Session represents an active or completed session.
type Session struct {
	ID              string        `json:"id"`
	Repo            string        `json:"repo"`
	TodoID          string        `json:"todo_id"`
	WorkspaceName   string        `json:"workspace_name"`
	Status          SessionStatus `json:"status"`
	Topic           string        `json:"topic"`
	StartedAt       time.Time     `json:"started_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	CompletedAt     time.Time     `json:"completed_at,omitempty"`
	ExitCode        *int          `json:"exit_code,omitempty"`
	DurationSeconds int           `json:"duration_seconds,omitempty"`
}
