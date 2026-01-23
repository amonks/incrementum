package workspace

import (
	"errors"

	statestore "github.com/amonks/incrementum/internal/state"
)

var (
	// ErrWorkspaceRootNotFound indicates a path is not in a jj workspace.
	ErrWorkspaceRootNotFound = errors.New("workspace root not found")
	// ErrRepoPathNotFound indicates a workspace is tracked but missing repo info.
	ErrRepoPathNotFound = statestore.ErrRepoPathNotFound
	// ErrOpencodeSessionNotFound indicates the requested session is missing.
	ErrOpencodeSessionNotFound = errors.New("opencode session not found")
	// ErrAmbiguousOpencodeSessionIDPrefix indicates a prefix matches multiple sessions.
	ErrAmbiguousOpencodeSessionIDPrefix = errors.New("ambiguous opencode session id prefix")
	// ErrOpencodeSessionNotActive indicates a session is not active.
	ErrOpencodeSessionNotActive = errors.New("opencode session is not active")
	// ErrOpencodeDaemonNotFound indicates the requested daemon is missing.
	ErrOpencodeDaemonNotFound = errors.New("opencode daemon not found")
)
