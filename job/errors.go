package job

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidStatus indicates a job status is invalid.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrInvalidStage indicates a job stage is invalid.
	ErrInvalidStage = errors.New("invalid stage")
	// ErrInvalidFeedbackFormat indicates a feedback file could not be parsed.
	ErrInvalidFeedbackFormat = errors.New("invalid feedback format")
	// ErrJobInterrupted indicates the job was interrupted.
	ErrJobInterrupted = errors.New("job interrupted")
	// ErrJobNotFound indicates the requested job is missing.
	ErrJobNotFound = errors.New("job not found")
	// ErrAmbiguousJobIDPrefix indicates a prefix matches multiple jobs.
	ErrAmbiguousJobIDPrefix = errors.New("ambiguous job id prefix")
)

func formatInvalidStatusError(status Status) error {
	return fmt.Errorf("%w: %q (valid: %s)", ErrInvalidStatus, status, validStatusList())
}

func formatInvalidStageError(stage Stage) error {
	return fmt.Errorf("%w: %q (valid: %s)", ErrInvalidStage, stage, validStageList())
}

func validStatusList() string {
	statuses := ValidStatuses()
	values := make([]string, 0, len(statuses))
	for _, status := range statuses {
		values = append(values, string(status))
	}
	return strings.Join(values, ", ")
}

func validStageList() string {
	stages := ValidStages()
	values := make([]string, 0, len(stages))
	for _, stage := range stages {
		values = append(values, string(stage))
	}
	return strings.Join(values, ", ")
}
