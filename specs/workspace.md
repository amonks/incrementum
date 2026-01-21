# Workspace Pool

## Overview
The workspace pool manages a shared set of jujutsu workspaces for a repository. It hands out leases to callers, reuses released workspaces, and persists workspace state so multiple processes can coordinate safely.

## Architecture
- `workspace.Pool` is the public API for acquiring, releasing, listing, and destroying workspaces.
- State is persisted in a JSON file managed by `stateStore` with an advisory file lock to serialize updates.
- Workspaces live under a shared base directory (`~/.local/share/incrementum/workspaces` by default). State lives under `~/.local/state/incrementum` by default.
- Jujutsu operations are delegated to `internal/jj` (workspace add/forget, edit, and new change).
- Configuration hooks are loaded from `.incr.toml` via `internal/config` and executed on each acquire.

## State Model
- State file is `state.json` with two maps:
  - `repos`: maps repo names to their source paths.
  - `workspaces`: maps `repoName/workspaceName` to workspace info.
- A repo name is a sanitized version of the source path, lowercased, with path separators converted to hyphens. Collisions add a numeric suffix.
- Workspace names are sequential `ws-###` values allocated per repo.
- Workspace info tracks: path, repo name, purpose, status, acquisition PID/time, and provisioning status.

## Workspace Lifecycle
### Acquire
- Defaults: `Rev` defaults to `@`.
- On acquire, the state store does the following under a lock:
  - Reuse the first available workspace for the repo when possible.
  - Otherwise allocate a new `ws-###` name and mark it acquired.
- If a new workspace is allocated, `jj workspace add` is executed and the workspace directory is created.
- The workspace is checked out to the requested revision with `jj edit`.
- A new change is created and the workspace is edited back to the requested revision to ensure a clean release change.
- `.incr.toml` is loaded from the source repo and the workspace `on-create` hook runs for every acquire (including reuse).
- A workspace is marked `Provisioned` once the hooks run successfully.

### Release
- Release creates a new change at `root()` to reset the workspace state.
- The workspace remains on disk, but its status is marked `available`, and purpose and acquisition metadata are cleared.

### List
- Listing returns every workspace for a repo.

### Destroy All
- Destroy-all removes workspaces for a repo from state, forgets each workspace from jj (best-effort), deletes the workspace directories, and removes the repo workspaces directory if empty.

## Repo Resolution
- `RepoRoot(path)` returns the jj root for any path.
- `RepoRootFromPath(path)` resolves a workspace path back to the source repo using state when possible.
- If the path is inside the workspace pool directory but no repo mapping exists, `ErrRepoPathNotFound` is returned.

## CLI Commands
- `ii workspace acquire [--rev <rev>] --purpose <text>`: acquire or create a workspace; prints the workspace path.
- `ii workspace release [name]`: release the named workspace (or current workspace when omitted).
- `ii workspace list [--json] [--all]`: list workspaces for the current repo.
- `ii workspace destroy-all`: remove all workspaces for the current repo.
