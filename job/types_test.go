package job

import (
	"testing"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestAliasesMatchJobModel(t *testing.T) {
	var status Status = statestore.JobStatusActive
	if status != StatusActive {
		t.Fatalf("expected status alias to match model")
	}

	var stage Stage = statestore.JobStageImplementing
	if stage != StageImplementing {
		t.Fatalf("expected stage alias to match model")
	}

	var item Job = statestore.Job{}
	if item.ID != "" {
		t.Fatalf("expected job alias to match model")
	}
}

func TestValidStagesReturnsModelValues(t *testing.T) {
	stages := ValidStages()
	if len(stages) != len(statestore.ValidJobStages()) {
		t.Fatalf("expected %d stages, got %d", len(statestore.ValidJobStages()), len(stages))
	}
}

func TestValidStatusesReturnsModelValues(t *testing.T) {
	statuses := ValidStatuses()
	if len(statuses) != len(statestore.ValidJobStatuses()) {
		t.Fatalf("expected %d statuses, got %d", len(statestore.ValidJobStatuses()), len(statuses))
	}
}
