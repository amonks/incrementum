package workspace

import (
	statestore "github.com/amonks/incrementum/internal/state"
)

// Status represents the state of a workspace.
type Status = statestore.WorkspaceStatus

const (
	// StatusAvailable indicates the workspace is free to be acquired.
	StatusAvailable Status = statestore.WorkspaceStatusAvailable

	// StatusAcquired indicates the workspace is currently in use.
	StatusAcquired Status = statestore.WorkspaceStatusAcquired
)

// ErrRepoPathNotFound indicates a workspace is tracked but missing repo info.
var ErrRepoPathNotFound = statestore.ErrRepoPathNotFound
