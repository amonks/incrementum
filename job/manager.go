package job

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

// OpenOptions configures a job manager.
type OpenOptions struct {
	// StateDir is the directory where job state is stored.
	StateDir string
}

// Manager provides access to job state for a repo.
type Manager struct {
	repoPath   string
	stateStore *statestore.Store
}

// Open opens a job manager for the given repo.
func Open(repoPath string, opts OpenOptions) (*Manager, error) {
	stateDir, err := resolveStateDir(opts)
	if err != nil {
		return nil, err
	}

	return &Manager{
		repoPath:   repoPath,
		stateStore: statestore.NewStore(stateDir),
	}, nil
}

// Create stores a new job with active status and implementing stage.
func (m *Manager) Create(todoID, sessionID string, startedAt time.Time) (Job, error) {
	if strings.TrimSpace(todoID) == "" {
		return Job{}, fmt.Errorf("todo id is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return Job{}, fmt.Errorf("session id is required")
	}

	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return Job{}, fmt.Errorf("get repo name: %w", err)
	}

	jobID := GenerateID(todoID, startedAt)
	created := Job{
		ID:        jobID,
		Repo:      repoName,
		TodoID:    todoID,
		SessionID: sessionID,
		Stage:     StageImplementing,
		Status:    StatusActive,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}

	err = m.stateStore.Update(func(st *statestore.State) error {
		st.Jobs[repoName+"/"+jobID] = created
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return created, nil
}

// UpdateOptions configures job updates.
// Nil fields mean "do not update".
type UpdateOptions struct {
	Stage                 *Stage
	Status                *Status
	Feedback              *string
	AppendOpencodeSession *OpencodeSession
}

// Update updates an existing job by id or prefix.
func (m *Manager) Update(jobID string, opts UpdateOptions, updatedAt time.Time) (Job, error) {
	if strings.TrimSpace(jobID) == "" {
		return Job{}, ErrJobNotFound
	}

	if opts.Stage != nil {
		normalized := Stage(strings.ToLower(string(*opts.Stage)))
		opts.Stage = &normalized
		if !opts.Stage.IsValid() {
			return Job{}, formatInvalidStageError(*opts.Stage)
		}
	}
	if opts.Status != nil {
		normalized := Status(strings.ToLower(string(*opts.Status)))
		opts.Status = &normalized
		if !opts.Status.IsValid() {
			return Job{}, formatInvalidStatusError(*opts.Status)
		}
	}

	found, err := m.Find(jobID)
	if err != nil {
		return Job{}, err
	}

	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	var updated Job
	err = m.stateStore.Update(func(st *statestore.State) error {
		key := found.Repo + "/" + found.ID
		job, ok := st.Jobs[key]
		if !ok {
			return ErrJobNotFound
		}
		if opts.Stage != nil {
			job.Stage = *opts.Stage
		}
		if opts.Status != nil {
			job.Status = *opts.Status
			if job.Status != StatusActive {
				job.CompletedAt = updatedAt
			}
		}
		if opts.Feedback != nil {
			job.Feedback = *opts.Feedback
		}
		if opts.AppendOpencodeSession != nil {
			job.OpencodeSessions = append(job.OpencodeSessions, *opts.AppendOpencodeSession)
		}
		job.UpdatedAt = updatedAt
		st.Jobs[key] = job
		updated = job
		return nil
	})
	if err != nil {
		return Job{}, err
	}

	return updated, nil
}

// ListFilter configures which jobs to return.
type ListFilter struct {
	// Status filters by exact status match.
	Status *Status
	// IncludeAll includes jobs regardless of status.
	IncludeAll bool
}

// List returns jobs for the repo.
func (m *Manager) List(filter ListFilter) ([]Job, error) {
	if filter.Status != nil {
		normalized := Status(strings.ToLower(string(*filter.Status)))
		filter.Status = &normalized
		if !filter.Status.IsValid() {
			return nil, formatInvalidStatusError(*filter.Status)
		}
	}

	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	st, err := m.stateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	items := make([]Job, 0)
	for _, job := range st.Jobs {
		if job.Repo != repoName {
			continue
		}
		if filter.Status != nil {
			if job.Status != *filter.Status {
				continue
			}
		} else if !filter.IncludeAll && job.Status != StatusActive {
			continue
		}
		items = append(items, job)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})

	return items, nil
}

// Find returns the job with the given id or prefix for the repo.
func (m *Manager) Find(jobID string) (Job, error) {
	if jobID == "" {
		return Job{}, ErrJobNotFound
	}

	repoName, err := m.stateStore.GetOrCreateRepoName(m.repoPath)
	if err != nil {
		return Job{}, fmt.Errorf("get repo name: %w", err)
	}

	st, err := m.stateStore.Load()
	if err != nil {
		return Job{}, fmt.Errorf("load state: %w", err)
	}

	needle := strings.ToLower(jobID)
	var match *Job
	for _, job := range st.Jobs {
		if job.Repo != repoName {
			continue
		}
		idLower := strings.ToLower(job.ID)
		if idLower != needle && !strings.HasPrefix(idLower, needle) {
			continue
		}
		if match != nil && !strings.EqualFold(match.ID, job.ID) {
			return Job{}, fmt.Errorf("%w: %s", ErrAmbiguousJobIDPrefix, jobID)
		}
		matched := job
		match = &matched
	}

	if match == nil {
		return Job{}, ErrJobNotFound
	}

	return *match, nil
}

func resolveStateDir(opts OpenOptions) (string, error) {
	if opts.StateDir != "" {
		return opts.StateDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".local", "state", "incrementum"), nil
}
