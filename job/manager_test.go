package job

import (
	"errors"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestManager_CreateAndFind(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/my-repo"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2025, 4, 10, 8, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-123", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	expectedID := GenerateID("todo-123", startedAt)
	if created.ID != expectedID {
		t.Fatalf("expected job id %q, got %q", expectedID, created.ID)
	}

	stateStore := statestore.NewStore(tmpDir)
	repoSlug, err := stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	if created.Repo != repoSlug {
		t.Fatalf("expected repo %q, got %q", repoSlug, created.Repo)
	}
	if created.TodoID != "todo-123" {
		t.Fatalf("expected todo id todo-123, got %q", created.TodoID)
	}
	if created.Status != StatusActive {
		t.Fatalf("expected status active, got %q", created.Status)
	}
	if created.Stage != StageImplementing {
		t.Fatalf("expected stage implementing, got %q", created.Stage)
	}
	if !created.CreatedAt.Equal(startedAt) {
		t.Fatalf("expected created at %v, got %v", startedAt, created.CreatedAt)
	}
	if !created.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started at %v, got %v", startedAt, created.StartedAt)
	}
	if !created.UpdatedAt.Equal(startedAt) {
		t.Fatalf("expected updated at %v, got %v", startedAt, created.UpdatedAt)
	}

	found, err := manager.Find(created.ID[:6])
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("expected job id %q, got %q", created.ID, found.ID)
	}
}

func TestManager_Find_PrefixAmbiguous(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/ambiguous"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	store := statestore.NewStore(tmpDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}

	startedAt := time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC)
	jobA := statestore.Job{
		ID:        "alpha-123",
		Repo:      repoSlug,
		TodoID:    "todo-1",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	jobB := statestore.Job{
		ID:        "alpha-456",
		Repo:      repoSlug,
		TodoID:    "todo-2",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt.Add(2 * time.Minute),
		StartedAt: startedAt.Add(2 * time.Minute),
		UpdatedAt: startedAt.Add(2 * time.Minute),
	}

	if err := insertJob(store, repoSlug, jobA); err != nil {
		t.Fatalf("insert jobA: %v", err)
	}
	if err := insertJob(store, repoSlug, jobB); err != nil {
		t.Fatalf("insert jobB: %v", err)
	}

	_, err = manager.Find("alpha")
	if err == nil {
		t.Fatalf("expected error for ambiguous prefix")
	}
	if !errors.Is(err, ErrAmbiguousJobIDPrefix) {
		t.Fatalf("expected ErrAmbiguousJobIDPrefix, got %v", err)
	}
}

func TestManager_List_Filtering(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/listing"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	store := statestore.NewStore(tmpDir)
	repoSlug, err := store.GetOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("repo slug: %v", err)
	}
	otherRepo, err := store.GetOrCreateRepoName("/Users/test/other")
	if err != nil {
		t.Fatalf("other repo slug: %v", err)
	}

	startedAt := time.Date(2025, 5, 10, 9, 0, 0, 0, time.UTC)
	activeJob := statestore.Job{
		ID:        "job-active",
		Repo:      repoSlug,
		TodoID:    "todo-active",
		Stage:     statestore.JobStageTesting,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	completedJob := statestore.Job{
		ID:          "job-completed",
		Repo:        repoSlug,
		TodoID:      "todo-completed",
		Stage:       statestore.JobStageCommitting,
		Status:      statestore.JobStatusCompleted,
		CreatedAt:   startedAt.Add(2 * time.Hour),
		StartedAt:   startedAt.Add(2 * time.Hour),
		UpdatedAt:   startedAt.Add(2 * time.Hour),
		CompletedAt: startedAt.Add(3 * time.Hour),
	}
	otherJob := statestore.Job{
		ID:        "job-other",
		Repo:      otherRepo,
		TodoID:    "todo-other",
		Stage:     statestore.JobStageImplementing,
		Status:    statestore.JobStatusActive,
		CreatedAt: startedAt.Add(30 * time.Minute),
		StartedAt: startedAt.Add(30 * time.Minute),
		UpdatedAt: startedAt.Add(30 * time.Minute),
	}

	if err := insertJob(store, repoSlug, activeJob); err != nil {
		t.Fatalf("insert active job: %v", err)
	}
	if err := insertJob(store, repoSlug, completedJob); err != nil {
		t.Fatalf("insert completed job: %v", err)
	}
	if err := insertJob(store, otherRepo, otherJob); err != nil {
		t.Fatalf("insert other job: %v", err)
	}

	listed, err := manager.List(ListFilter{})
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != activeJob.ID {
		t.Fatalf("expected only active job, got %v", listed)
	}

	allJobs, err := manager.List(ListFilter{IncludeAll: true})
	if err != nil {
		t.Fatalf("list all jobs: %v", err)
	}
	if len(allJobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(allJobs))
	}
	if allJobs[0].ID != activeJob.ID || allJobs[1].ID != completedJob.ID {
		t.Fatalf("expected jobs ordered by start time, got %v", allJobs)
	}

	status := Status("COMPLETED")
	completedOnly, err := manager.List(ListFilter{Status: &status})
	if err != nil {
		t.Fatalf("list completed jobs: %v", err)
	}
	if len(completedOnly) != 1 || completedOnly[0].ID != completedJob.ID {
		t.Fatalf("expected only completed job, got %v", completedOnly)
	}
}

func TestManager_Update(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/update"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2025, 6, 1, 9, 30, 0, 0, time.UTC)
	created, err := manager.Create("todo-456", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	updatedAt := startedAt.Add(2 * time.Hour)
	stage := Stage("TESTING")
	status := Status("FAILED")
	feedback := "tests failed"
	opencode := OpencodeSession{Purpose: "implement", ID: "oc-123"}

	updated, err := manager.Update(created.ID[:6], UpdateOptions{
		Stage:                 &stage,
		Status:                &status,
		Feedback:              &feedback,
		AppendOpencodeSession: &opencode,
	}, updatedAt)
	if err != nil {
		t.Fatalf("update job: %v", err)
	}

	if updated.Stage != StageTesting {
		t.Fatalf("expected stage testing, got %q", updated.Stage)
	}
	if updated.Status != StatusFailed {
		t.Fatalf("expected status failed, got %q", updated.Status)
	}
	if updated.Feedback != feedback {
		t.Fatalf("expected feedback %q, got %q", feedback, updated.Feedback)
	}
	if len(updated.OpencodeSessions) != 1 {
		t.Fatalf("expected 1 opencode session, got %d", len(updated.OpencodeSessions))
	}
	if updated.OpencodeSessions[0] != opencode {
		t.Fatalf("expected opencode session %+v, got %+v", opencode, updated.OpencodeSessions[0])
	}
	if !updated.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("expected updated at %v, got %v", updatedAt, updated.UpdatedAt)
	}
	if !updated.CompletedAt.Equal(updatedAt) {
		t.Fatalf("expected completed at %v, got %v", updatedAt, updated.CompletedAt)
	}

	stored, err := manager.Find(created.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	if stored.Status != StatusFailed {
		t.Fatalf("expected stored status failed, got %q", stored.Status)
	}
}

func TestManager_Update_InvalidStage(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/update-invalid"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2025, 6, 2, 11, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-789", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	stage := Stage("unknown")
	_, err = manager.Update(created.ID, UpdateOptions{Stage: &stage}, startedAt.Add(time.Minute))
	if err == nil {
		t.Fatalf("expected invalid stage error")
	}
	if !errors.Is(err, ErrInvalidStage) {
		t.Fatalf("expected ErrInvalidStage, got %v", err)
	}
}

func TestManager_Update_InvalidStatus(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := "/Users/test/update-invalid-status"
	manager, err := Open(repoPath, OpenOptions{StateDir: tmpDir})
	if err != nil {
		t.Fatalf("open manager: %v", err)
	}

	startedAt := time.Date(2025, 6, 2, 12, 0, 0, 0, time.UTC)
	created, err := manager.Create("todo-790", startedAt, CreateOptions{})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	status := Status("unknown")
	_, err = manager.Update(created.ID, UpdateOptions{Status: &status}, startedAt.Add(time.Minute))
	if err == nil {
		t.Fatalf("expected invalid status error")
	}
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func insertJob(store *statestore.Store, repoSlug string, item statestore.Job) error {
	return store.Update(func(st *statestore.State) error {
		st.Jobs[repoSlug+"/"+item.ID] = item
		return nil
	})
}
