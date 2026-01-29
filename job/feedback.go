package job

import (
	"errors"
	"fmt"
	"os"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// ReviewOutcome captures the outcome of opencode review feedback.
type ReviewOutcome string

const (
	// ReviewOutcomeAccept indicates the review accepted changes.
	ReviewOutcomeAccept ReviewOutcome = "ACCEPT"
	// ReviewOutcomeAbandon indicates the review rejected the work entirely.
	ReviewOutcomeAbandon ReviewOutcome = "ABANDON"
	// ReviewOutcomeRequestChanges indicates the review requested changes.
	ReviewOutcomeRequestChanges ReviewOutcome = "REQUEST_CHANGES"
)

// ReviewFeedback is parsed feedback from the opencode review stage.
type ReviewFeedback struct {
	Outcome ReviewOutcome
	Details string
}

// ReadReviewFeedback loads feedback from a file.
// Missing files are treated as ACCEPT.
func ReadReviewFeedback(path string) (ReviewFeedback, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ReviewFeedback{Outcome: ReviewOutcomeAccept}, nil
		}
		return ReviewFeedback{}, fmt.Errorf("read feedback: %w", err)
	}
	removeErr := removeFileIfExists(path)
	if removeErr != nil {
		removeErr = fmt.Errorf("remove feedback: %w", removeErr)
	}

	feedback, parseErr := ParseReviewFeedback(string(data))
	if removeErr != nil {
		if parseErr != nil {
			return ReviewFeedback{}, errors.Join(parseErr, removeErr)
		}
		return feedback, removeErr
	}
	return feedback, parseErr
}

// ParseReviewFeedback parses the feedback file contents.
func ParseReviewFeedback(contents string) (ReviewFeedback, error) {
	lines := strings.Split(contents, "\n")

	for i, line := range lines {
		lines[i] = internalstrings.TrimTrailingCarriageReturn(line)
	}

	firstLine := internalstrings.TrimSpace(lines[0])
	if firstLine == "" {
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	var outcome ReviewOutcome
	switch {
	case strings.EqualFold(firstLine, string(ReviewOutcomeAccept)):
		return ReviewFeedback{Outcome: ReviewOutcomeAccept}, nil
	case strings.EqualFold(firstLine, string(ReviewOutcomeAbandon)):
		outcome = ReviewOutcomeAbandon
	case strings.EqualFold(firstLine, string(ReviewOutcomeRequestChanges)):
		outcome = ReviewOutcomeRequestChanges
	default:
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	blankIndex := -1
	for i := 1; i < len(lines); i++ {
		if internalstrings.IsBlank(lines[i]) {
			blankIndex = i
			break
		}
	}
	if blankIndex == -1 {
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	details := strings.Join(lines[blankIndex+1:], "\n")
	details = internalstrings.TrimTrailingNewlines(details)
	if details == "" {
		return ReviewFeedback{}, ErrInvalidFeedbackFormat
	}

	return ReviewFeedback{Outcome: outcome, Details: details}, nil
}
