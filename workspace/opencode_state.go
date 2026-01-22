package workspace

import (
	statestore "github.com/amonks/incrementum/internal/state"
)

// OpencodeDaemonStatus represents the state of an opencode daemon.
type OpencodeDaemonStatus = statestore.OpencodeDaemonStatus

const (
	// OpencodeDaemonRunning indicates the daemon is running.
	OpencodeDaemonRunning OpencodeDaemonStatus = statestore.OpencodeDaemonRunning
	// OpencodeDaemonStopped indicates the daemon is stopped.
	OpencodeDaemonStopped OpencodeDaemonStatus = statestore.OpencodeDaemonStopped
)

// OpencodeDaemon stores daemon state for a repo.
type OpencodeDaemon = statestore.OpencodeDaemon

// OpencodeSessionStatus represents the state of an opencode session.
type OpencodeSessionStatus = statestore.OpencodeSessionStatus

const (
	// OpencodeSessionActive indicates the session is active.
	OpencodeSessionActive OpencodeSessionStatus = statestore.OpencodeSessionActive
	// OpencodeSessionCompleted indicates the session completed successfully.
	OpencodeSessionCompleted OpencodeSessionStatus = statestore.OpencodeSessionCompleted
	// OpencodeSessionFailed indicates the session failed.
	OpencodeSessionFailed OpencodeSessionStatus = statestore.OpencodeSessionFailed
	// OpencodeSessionKilled indicates the session was terminated.
	OpencodeSessionKilled OpencodeSessionStatus = statestore.OpencodeSessionKilled
)

// OpencodeSession stores session state for a repo.
type OpencodeSession = statestore.OpencodeSession
