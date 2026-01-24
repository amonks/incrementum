# Internal JJ

## Overview
The jj package wraps the `jj` CLI to provide Go-friendly helpers.

## Client Operations
- Repository init: `Init` runs `jj git init`.
- Workspace operations: `WorkspaceRoot`, `WorkspaceAdd`, `WorkspaceList`, `WorkspaceForget`, `WorkspaceUpdateStale`.
- Change operations: `Edit`, `NewChange`, `CurrentChangeID`, `ChangeIDAt`, `Snapshot`, `Describe`.
- `Describe` uses `jj describe --stdin` to avoid long argument lists.
- `Commit` is implemented as `Describe` followed by `NewChange`.
- Bookmark operations: `BookmarkList`, `BookmarkCreate`.

## Error Handling
- CLI output is included in errors to help diagnose failures.
