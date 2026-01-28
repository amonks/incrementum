// Package state manages the shared incrementum state file.
//
// The state file (~/.local/state/incrementum/state.json) stores persistent
// state for workspaces, opencode sessions, and jobs. All access is
// serialized through file locking to allow safe concurrent access from
// multiple processes.
package state

import (
	"time"

	"github.com/amonks/incrementum/internal/validation"
)

// State represents the persisted state file.
type State struct {
	Repos            map[string]RepoInfo        `json:"repos"`
	Workspaces       map[string]WorkspaceInfo   `json:"workspaces"`
	OpencodeSessions map[string]OpencodeSession `json:"opencode_sessions"`
	Jobs             map[string]Job             `json:"jobs"`
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

// ValidWorkspaceStatuses returns all valid workspace status values.
func ValidWorkspaceStatuses() []WorkspaceStatus {
	return []WorkspaceStatus{WorkspaceStatusAvailable, WorkspaceStatusAcquired}
}

// IsValid returns true if the status is a known value.
func (s WorkspaceStatus) IsValid() bool {
	return validation.IsValidValue(s, ValidWorkspaceStatuses())
}

// WorkspaceInfo stores information about a workspace.
type WorkspaceInfo struct {
	Name          string          `json:"name"`
	Repo          string          `json:"repo"`
	Path          string          `json:"path"`
	Purpose       string          `json:"purpose,omitempty"`
	Rev           string          `json:"rev,omitempty"`
	Status        WorkspaceStatus `json:"status"`
	AcquiredByPID int             `json:"acquired_by_pid,omitempty"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at,omitempty"`
	AcquiredAt    time.Time       `json:"acquired_at,omitempty"`
	Provisioned   bool            `json:"provisioned"`
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
	CreatedAt       time.Time             `json:"created_at,omitempty"`
	StartedAt       time.Time             `json:"started_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	CompletedAt     time.Time             `json:"completed_at,omitempty"`
	ExitCode        *int                  `json:"exit_code,omitempty"`
	DurationSeconds int                   `json:"duration_seconds,omitempty"`
	LogPath         string                `json:"log_path,omitempty"`
}

// JobStage represents the current workflow stage for a job.
type JobStage string

const (
	// JobStageImplementing indicates the opencode implementation stage.
	JobStageImplementing JobStage = "implementing"
	// JobStageTesting indicates the test execution stage.
	JobStageTesting JobStage = "testing"
	// JobStageReviewing indicates the opencode review stage.
	JobStageReviewing JobStage = "reviewing"
	// JobStageCommitting indicates the commit message generation stage.
	JobStageCommitting JobStage = "committing"
)

// ValidJobStages returns all valid job stage values.
func ValidJobStages() []JobStage {
	return []JobStage{JobStageImplementing, JobStageTesting, JobStageReviewing, JobStageCommitting}
}

// IsValid returns true if the stage is a known value.
func (s JobStage) IsValid() bool {
	return validation.IsValidValue(s, ValidJobStages())
}

// JobStatus represents the lifecycle status for a job.
type JobStatus string

const (
	// JobStatusActive indicates the job is still running.
	JobStatusActive JobStatus = "active"
	// JobStatusCompleted indicates the job completed successfully.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates the job failed.
	JobStatusFailed JobStatus = "failed"
	// JobStatusAbandoned indicates the job was abandoned.
	JobStatusAbandoned JobStatus = "abandoned"
)

// ValidJobStatuses returns all valid job status values.
func ValidJobStatuses() []JobStatus {
	return []JobStatus{JobStatusActive, JobStatusCompleted, JobStatusFailed, JobStatusAbandoned}
}

// IsValid returns true if the status is a known value.
func (s JobStatus) IsValid() bool {
	return validation.IsValidValue(s, ValidJobStatuses())
}

// JobOpencodeSession tracks an opencode session started by a job.
type JobOpencodeSession struct {
	Purpose string `json:"purpose"`
	ID      string `json:"id"`
}

// Job stores job state for a repo.
type Job struct {
	ID                  string               `json:"id"`
	Repo                string               `json:"repo"`
	TodoID              string               `json:"todo_id"`
	Agent               string               `json:"agent"`
	ImplementationModel string               `json:"implementation_model,omitempty"`
	CodeReviewModel     string               `json:"code_review_model,omitempty"`
	ProjectReviewModel  string               `json:"project_review_model,omitempty"`
	Stage               JobStage             `json:"stage"`
	Feedback            string               `json:"feedback,omitempty"`
	OpencodeSessions    []JobOpencodeSession `json:"opencode_sessions,omitempty"`
	Status              JobStatus            `json:"status"`
	CreatedAt           time.Time            `json:"created_at,omitempty"`
	StartedAt           time.Time            `json:"started_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
	CompletedAt         time.Time            `json:"completed_at,omitempty"`
}
