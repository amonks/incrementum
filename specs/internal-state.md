# Internal State

## Overview
The state package manages the shared incrementum state file (`~/.local/state/incrementum/state.json`). It provides persistence and locking for workspaces, opencode sessions, and jobs.

## State File Structure
The state file contains:
- `repos`: maps repo names to source paths
- `workspaces`: maps workspace keys to workspace info
- `opencode_sessions`: maps session keys to opencode session records
- `jobs`: maps job ids to job records

## Types

### WorkspaceInfo
- `name`, `repo`, `path`, `purpose`, `status`, `created_at`, `updated_at`, `acquired_by_pid`, `acquired_at`, `provisioned`
- Status: `available` or `acquired`

### OpencodeSession
- `id`, `repo`, `status`, `prompt`, `created_at`, `started_at`, `updated_at`, `completed_at`, `exit_code`, `duration_seconds`
- Status: `active`, `completed`, `failed`, or `killed`

### Job
- `id`, `repo`, `todo_id`, `stage`, `feedback`, `opencode_sessions`, `status`, `created_at`, `started_at`, `updated_at`, `completed_at`
- Stage: `implementing`, `testing`, `reviewing`, or `committing`
- Status: `active`, `completed`, `failed`, or `abandoned`

## Locking
All state updates use advisory file locking via `state.lock` to serialize concurrent access from multiple processes.

## API
- `NewStore(dir)`: create a store for the given directory
- `Load()`: read current state
- `Save(state)`: write state atomically, skipping disk writes when no changes
- `Update(fn)`: read-modify-write with locking
- `GetOrCreateRepoName(path)`: get or create repo name for path
- `RepoPathForWorkspace(wsPath)`: resolve workspace path to source repo
- `SanitizeRepoName(path)`: convert path to safe repo name
