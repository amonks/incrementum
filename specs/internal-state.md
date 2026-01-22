# Internal State

## Overview
The state package manages the shared incrementum state file (`~/.local/state/incrementum/state.json`). It provides persistence and locking for workspaces, sessions, and opencode daemons.

## State File Structure
The state file contains:
- `repos`: maps repo names to source paths
- `workspaces`: maps workspace keys to workspace info
- `sessions`: maps session keys to session records
- `opencode_daemons`: maps repo names to daemon state
- `opencode_sessions`: maps session keys to opencode session records

## Types

### WorkspaceInfo
- `name`, `repo`, `path`, `purpose`, `status`, `acquired_by_pid`, `acquired_at`, `provisioned`
- Status: `available` or `acquired`

### Session
- `id`, `repo`, `todo_id`, `workspace_name`, `status`, `topic`, `started_at`, `updated_at`, `completed_at`, `exit_code`, `duration_seconds`
- Status: `active`, `completed`, or `failed`

### OpencodeDaemon
- `repo`, `status`, `started_at`, `updated_at`, `pid`, `host`, `port`, `log_path`
- Status: `running` or `stopped`

### OpencodeSession
- `id`, `repo`, `status`, `prompt`, `started_at`, `updated_at`, `completed_at`, `exit_code`, `duration_seconds`, `log_path`
- Status: `active`, `completed`, `failed`, or `killed`

## Locking
All state updates use advisory file locking via `state.lock` to serialize concurrent access from multiple processes.

## API
- `NewStore(dir)`: create a store for the given directory
- `Load()`: read current state
- `Save(state)`: write state atomically
- `Update(fn)`: read-modify-write with locking
- `GetOrCreateRepoName(path)`: get or create repo name for path
- `RepoPathForWorkspace(wsPath)`: resolve workspace path to source repo
- `SanitizeRepoName(path)`: convert path to safe repo name
