// Package state manages the shared incrementum state file.
//
// The state file (~/.local/state/incrementum/state.json) stores persistent
// state for workspaces, sessions, and opencode daemons. All access is
// serialized through file locking to allow safe concurrent access from
// multiple processes.
package state

import "time"

// State represents the persisted state file.
type State struct {
	Repos            map[string]RepoInfo        `json:"repos"`
	Workspaces       map[string]WorkspaceInfo   `json:"workspaces"`
	Sessions         map[string]Session         `json:"sessions"`
	OpencodeDaemons  map[string]OpencodeDaemon  `json:"opencode_daemons"`
	OpencodeSessions map[string]OpencodeSession `json:"opencode_sessions"`
}

// RepoInfo stores information about a tracked repository.
type RepoInfo struct {
	SourcePath string `json:"source_path"`
}

// WorkspaceStatus represents the state of a workspace.
type WorkspaceStatus string

const (
	// WorkspaceStatusAvailable indicates the workspace is free to be acquired.
	WorkspaceStatusAvailable WorkspaceStatus = "available"
	// WorkspaceStatusAcquired indicates the workspace is currently in use.
	WorkspaceStatusAcquired WorkspaceStatus = "acquired"
)

// WorkspaceInfo stores information about a workspace.
type WorkspaceInfo struct {
	Name          string          `json:"name"`
	Repo          string          `json:"repo"`
	Path          string          `json:"path"`
	Purpose       string          `json:"purpose,omitempty"`
	Status        WorkspaceStatus `json:"status"`
	AcquiredByPID int             `json:"acquired_by_pid,omitempty"`
	AcquiredAt    time.Time       `json:"acquired_at,omitempty"`
	Provisioned   bool            `json:"provisioned"`
}

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
