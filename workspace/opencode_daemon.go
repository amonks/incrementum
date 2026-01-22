package workspace

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

// ErrOpencodeDaemonNotFound indicates the requested daemon is missing.
var ErrOpencodeDaemonNotFound = errors.New("opencode daemon not found")

// RecordOpencodeDaemon stores a running opencode daemon for a repo.
func (p *Pool) RecordOpencodeDaemon(repoPath string, pid int, host string, port int, logPath string, startedAt time.Time) (OpencodeDaemon, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeDaemon{}, fmt.Errorf("get repo name: %w", err)
	}

	created := OpencodeDaemon{
		Repo:      repoName,
		Status:    OpencodeDaemonRunning,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
		PID:       pid,
		Host:      host,
		Port:      port,
		LogPath:   logPath,
	}

	err = p.stateStore.Update(func(st *statestore.State) error {
		st.OpencodeDaemons[repoName] = created
		return nil
	})
	if err != nil {
		return OpencodeDaemon{}, err
	}

	return created, nil
}

// FindOpencodeDaemon returns the daemon for the given repo.
func (p *Pool) FindOpencodeDaemon(repoPath string) (OpencodeDaemon, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeDaemon{}, fmt.Errorf("get repo name: %w", err)
	}

	st, err := p.stateStore.Load()
	if err != nil {
		return OpencodeDaemon{}, fmt.Errorf("load state: %w", err)
	}

	daemon, ok := st.OpencodeDaemons[repoName]
	if !ok {
		return OpencodeDaemon{}, ErrOpencodeDaemonNotFound
	}

	if daemon.Status == OpencodeDaemonRunning && daemon.PID > 0 && !processRunning(daemon.PID) {
		updatedAt := time.Now()
		err = p.stateStore.Update(func(st *statestore.State) error {
			current, ok := st.OpencodeDaemons[repoName]
			if !ok {
				return ErrOpencodeDaemonNotFound
			}
			current.Status = OpencodeDaemonStopped
			current.UpdatedAt = updatedAt
			st.OpencodeDaemons[repoName] = current
			daemon = current
			return nil
		})
		if err != nil {
			return OpencodeDaemon{}, err
		}
	}

	return daemon, nil
}

// StopOpencodeDaemon marks the daemon as stopped.
func (p *Pool) StopOpencodeDaemon(repoPath string, stoppedAt time.Time) (OpencodeDaemon, error) {
	repoName, err := p.stateStore.GetOrCreateRepoName(repoPath)
	if err != nil {
		return OpencodeDaemon{}, fmt.Errorf("get repo name: %w", err)
	}

	var updated OpencodeDaemon
	err = p.stateStore.Update(func(st *statestore.State) error {
		daemon, ok := st.OpencodeDaemons[repoName]
		if !ok {
			return ErrOpencodeDaemonNotFound
		}
		daemon.Status = OpencodeDaemonStopped
		daemon.UpdatedAt = stoppedAt
		st.OpencodeDaemons[repoName] = daemon
		updated = daemon
		return nil
	})
	if err != nil {
		return OpencodeDaemon{}, err
	}

	return updated, nil
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return true
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return errors.Is(err, syscall.EPERM)
	}
	return true
}
