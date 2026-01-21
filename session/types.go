package session

import "time"

// Status represents the session lifecycle state.
type Status string

const (
	// StatusActive indicates the session is in progress.
	StatusActive Status = "active"
	// StatusCompleted indicates the session completed successfully.
	StatusCompleted Status = "completed"
	// StatusFailed indicates the session failed.
	StatusFailed Status = "failed"
)

// ValidStatuses returns all valid session status values.
func ValidStatuses() []Status {
	return []Status{StatusActive, StatusCompleted, StatusFailed}
}

// IsValid returns true if the status is a known value.
func (s Status) IsValid() bool {
	for _, valid := range ValidStatuses() {
		if s == valid {
			return true
		}
	}
	return false
}

// Session captures session metadata for a todo.
type Session struct {
	ID              string    `json:"id"`
	Repo            string    `json:"repo"`
	TodoID          string    `json:"todo_id"`
	WorkspaceName   string    `json:"workspace_name"`
	Status          Status    `json:"status"`
	Topic           string    `json:"topic"`
	StartedAt       time.Time `json:"started_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	ExitCode        *int      `json:"exit_code,omitempty"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`
}
