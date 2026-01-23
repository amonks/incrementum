package job

import (
	"errors"
	"fmt"

	"github.com/amonks/incrementum/internal/validation"
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
	return fmt.Errorf("%w: %q (valid: %s)", ErrInvalidStatus, status, validation.FormatValidValues(ValidStatuses()))
}

func formatInvalidStageError(stage Stage) error {
	return fmt.Errorf("%w: %q (valid: %s)", ErrInvalidStage, stage, validation.FormatValidValues(ValidStages()))
}
