package job

import (
	statestore "github.com/amonks/incrementum/internal/state"
	"github.com/amonks/incrementum/todo"
)

// Status represents the job lifecycle state.
type Status = statestore.JobStatus

const (
	// StatusActive indicates the job is running.
	StatusActive Status = statestore.JobStatusActive
	// StatusCompleted indicates the job completed successfully.
	StatusCompleted Status = statestore.JobStatusCompleted
	// StatusFailed indicates the job failed.
	StatusFailed Status = statestore.JobStatusFailed
	// StatusAbandoned indicates the job was abandoned.
	StatusAbandoned Status = statestore.JobStatusAbandoned
)

// ValidStatuses returns all valid job status values.
func ValidStatuses() []Status {
	return statestore.ValidJobStatuses()
}

// Stage represents the job workflow stage.
type Stage = statestore.JobStage

const (
	// StageImplementing indicates the implementation stage.
	StageImplementing Stage = statestore.JobStageImplementing
	// StageTesting indicates the test execution stage.
	StageTesting Stage = statestore.JobStageTesting
	// StageReviewing indicates the review stage.
	StageReviewing Stage = statestore.JobStageReviewing
	// StageCommitting indicates the commit message stage.
	StageCommitting Stage = statestore.JobStageCommitting
)

// ValidStages returns all valid job stage values.
func ValidStages() []Stage {
	return statestore.ValidJobStages()
}

// OpencodeSession tracks opencode sessions spawned by a job.
type OpencodeSession = statestore.JobOpencodeSession

// Job captures job metadata for a todo.
type Job = statestore.Job

// StartInfo captures context when starting a job run.
type StartInfo struct {
	Workdir string
	Todo    todo.Todo
}
