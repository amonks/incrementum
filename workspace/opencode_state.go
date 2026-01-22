package workspace

import "time"

// OpencodeDaemonStatus represents the state of an opencode daemon.
type OpencodeDaemonStatus string

const (
	// OpencodeDaemonRunning indicates the daemon is running.
	OpencodeDaemonRunning OpencodeDaemonStatus = "running"
	// OpencodeDaemonStopped indicates the daemon is stopped.
	OpencodeDaemonStopped OpencodeDaemonStatus = "stopped"
)

// OpencodeDaemon stores daemon state for a repo.
type OpencodeDaemon struct {
	Repo      string               `json:"repo"`
	Status    OpencodeDaemonStatus `json:"status"`
	StartedAt time.Time            `json:"started_at"`
	UpdatedAt time.Time            `json:"updated_at"`
	PID       int                  `json:"pid,omitempty"`
	Host      string               `json:"host,omitempty"`
	Port      int                  `json:"port,omitempty"`
	LogPath   string               `json:"log_path,omitempty"`
}

// OpencodeSessionStatus represents the state of an opencode session.
type OpencodeSessionStatus string

const (
	// OpencodeSessionActive indicates the session is active.
	OpencodeSessionActive OpencodeSessionStatus = "active"
	// OpencodeSessionCompleted indicates the session completed successfully.
	OpencodeSessionCompleted OpencodeSessionStatus = "completed"
	// OpencodeSessionFailed indicates the session failed.
	OpencodeSessionFailed OpencodeSessionStatus = "failed"
	// OpencodeSessionKilled indicates the session was terminated.
	OpencodeSessionKilled OpencodeSessionStatus = "killed"
)

// OpencodeSession stores session state for a repo.
type OpencodeSession struct {
	ID              string                `json:"id"`
	Repo            string                `json:"repo"`
	Status          OpencodeSessionStatus `json:"status"`
	Prompt          string                `json:"prompt"`
	StartedAt       time.Time             `json:"started_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	CompletedAt     time.Time             `json:"completed_at,omitempty"`
	ExitCode        *int                  `json:"exit_code,omitempty"`
	DurationSeconds int                   `json:"duration_seconds,omitempty"`
	LogPath         string                `json:"log_path,omitempty"`
}

// OpencodeSessionAge computes the display age for an opencode session.
func OpencodeSessionAge(session OpencodeSession, now time.Time) time.Duration {
	if session.Status == OpencodeSessionActive {
		if session.StartedAt.IsZero() {
			return 0
		}
		return now.Sub(session.StartedAt)
	}

	if session.DurationSeconds > 0 {
		return time.Duration(session.DurationSeconds) * time.Second
	}

	if !session.CompletedAt.IsZero() && !session.StartedAt.IsZero() {
		return session.CompletedAt.Sub(session.StartedAt)
	}

	return 0
}
