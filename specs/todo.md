# Todo Store

## Overview

The todo subsystem provides a lightweight, Jira-like tracker scoped to a
jujutsu repository. Todos live in a dedicated store so they can be shared
across workspaces without polluting the code history.

## Architecture

- Storage lives in a special orphan change parented at `root()` and
  referenced by the jj bookmark `incr/tasks`.
- Writes access the store through a background workspace from the workspace
  pool; operations never mutate the user's working copy.
- Read-only access does not require a workspace. Reads use
  `jj file show -r incr/tasks <file>` to fetch JSONL data directly.
- Data is stored as JSONL files in the store workspace:
  - `todos.jsonl` holds one JSON object per todo.
  - `dependencies.jsonl` holds one JSON object per dependency.
- All writes are guarded by exclusive file locks, written to a temp file
  and atomically renamed. Each write snapshots the jj workspace to persist
  the change.
- `todo.Open` can create the store when missing, optionally prompting the
  user before creating the bookmark.
- `todo.Open` acquires a workspace with a purpose string from
  `OpenOptions.Purpose`, defaulting to `todo store`.
- `OpenOptions.ReadOnly` skips workspace acquisition and opens the store
  for read-only access.
- Prompting via stdin only happens when stdin is a TTY; non-interactive calls
  skip the prompt and proceed with creation unless a custom prompter is used.

## Data Model

### Todo

Fields (JSON keys):

- `id`: 8-character lowercase base32 identifier.
- `title`: required; must include non-whitespace characters; max length 500 characters.
- `description`: optional free text.
- `status`: `open`, `in_progress`, `closed`, `done`, or `tombstone`.
- `priority`: integer 0..4 (0 = critical, 4 = backlog).
- `type`: `task`, `bug`, or `feature`.
- `created_at`, `updated_at`: timestamps.
- `closed_at`: timestamp if closed or done.
- `deleted_at`: timestamp if tombstoned.
- `delete_reason`: optional reason when tombstoned.

### Dependency

Fields (JSON keys):

- `todo_id`: todo that owns the dependency.
- `depends_on_id`: todo that must be resolved first.
- `type`: `blocks` or `discovered-from`.
- `created_at`: timestamp.

## Semantics

### ID Generation

- IDs are derived from `title + RFC3339Nano timestamp`, hashed with SHA-256,
  then base32-encoded and lowercased.
- The store resolves user-provided IDs by case-insensitive prefix matching.
  Prefixes must be unambiguous; otherwise operations fail.

### Status + Timestamp Rules

- `open`/`in_progress`: `closed_at` must be empty; `deleted_at` must be empty.
- `closed`/`done`: `closed_at` must be set; `deleted_at` must be empty.
- `tombstone`: `deleted_at` must be set; `closed_at` must be empty;
  `delete_reason` is allowed only when tombstoned.

### Create

- Title is required and validated.
- CLI `todo create` expects the title via `--title`; it is not positional.
- Defaults: `type=task`, `priority=medium` (2), `status=open`.
- Type and dependency type inputs are case-insensitive and stored as lowercase.
- Editor mode is used by default only when no create fields are supplied; use `--edit` to force it or `--no-edit` to skip it.
- CLI description input via `--description -` / `--desc -` trims trailing CR/LF characters.
- Dependencies may be supplied as `type:id` pairs; each dependency must
  reference an existing todo.
- Dependency IDs accept the same case-insensitive prefix matching as other
  commands.

### Update

- Only fields explicitly provided are changed.
- When `todo update` runs in editor mode for multiple IDs, the CLI opens one editor session per todo.
- Editor mode is used by default only when no update fields are supplied; if update fields are provided, the editor opens only with `--edit`.
- CLI description input via `--description -` / `--desc -` trims trailing CR/LF characters.
- Status transitions automatically adjust timestamps:
  - `closed`/`done` sets `closed_at` and clears delete markers.
  - `open`/`in_progress` clears `closed_at` and delete markers.
  - `tombstone` clears `closed_at`; `deleted_at` must be set.
- Status and type inputs are case-insensitive and stored as lowercase.
- Updating `deleted_at` without `delete_reason` preserves any existing delete reason; clear it explicitly when needed.
- Reapplying the current status does not reset timestamps unless explicitly provided.
- `updated_at` always changes when a todo is updated.

### Close / Reopen / Start / Delete

- `close` sets status to `closed` and updates `closed_at`.
- `reopen` sets status to `open` and clears `closed_at`.
- `start` sets status to `in_progress` and clears `closed_at`.
- `delete` sets status to `tombstone`, sets `deleted_at`, clears `closed_at`,
  and optionally records a delete reason.
- Close/finish/reopen/start do not store reasons; only delete supports
  `delete_reason`.

### List

- Returns todos matching optional filters: status, priority, type, IDs,
  title substring, description substring.
- Priority filters must be within 0..4; invalid values return an error.
- Status and type filters are case-insensitive.
- Invalid status or type filters return errors listing valid values.
- Tombstones are excluded by default unless `IncludeTombstones` is set.
- Setting `Status=tombstone` implicitly includes tombstones in list results.
- CLI `todo list` includes tombstones when `--tombstones` is provided or when `--status tombstone` is specified.
- CLI `todo list` excludes `done` todos by default unless `--status` or `--all` is provided.
- When `todo list` is empty but matching `done` or `tombstone` todos exist, the CLI prints a hint to use `--all` and/or `--tombstones`.
- CLI ID highlighting uses the shortest unique prefix across all todos,
  including tombstones, so the display matches prefix resolution.
- All CLI outputs that show todo IDs (create/update logs, show/detail views,
  list/ready tables, dependency output) use the same prefix highlighting rules.
- CLI table output includes an `AGE` column formatted as `<count><unit>`, using
  `s`, `m`, `h`, or `d` based on recency.
- `AGE` uses `now - created_at`.
- When the todo store is missing, CLI `todo list` does not prompt to create it
  and returns an empty list.

### Show

- CLI detail output includes deleted timestamps and delete reasons when present.
- When the todo store is missing, CLI `todo show` does not prompt to create it
  and returns the store missing error.
- `Store.Show` returns todos in the same order as the requested IDs.

### Ready

- Returns `open` todos that have no unresolved `blocks` dependencies.
- A blocker is unresolved when the blocking todo is not `closed`, `done`, or `tombstone`.
- Results are ordered by priority (ascending), then type (bug, task, feature),
  then creation time (oldest first); an optional limit truncates the list.
- When the todo store is missing, CLI `todo ready` does not prompt to create it
  and returns an empty list.

### Dependencies

- `blocks` means `depends_on_id` must be closed before `todo_id` is ready.
- `discovered-from` links related work but does not affect readiness.
- Self-dependencies and duplicates are rejected.
- Invalid dependency type inputs return an error listing valid values.
- Dependency trees are computed by walking dependencies from a root todo;
  cycles are avoided by tracking the current traversal path so shared
  dependencies can appear under each branch.
- When the todo store is missing, CLI dependency tree output does not prompt to
  create it and returns the store missing error.

## CLI Mapping

The CLI mirrors the store API:

- `todo create` -> `Store.Create`
- `todo update` -> `Store.Update`
- `todo start` -> `Store.Start`
- `todo close` -> `Store.Close`
- `todo finish` (`todo done`) -> `Store.Finish`
- `todo reopen` -> `Store.Reopen`
- `todo delete` -> `Store.Delete`
- `todo show` -> `Store.Show`
- `todo list` -> `Store.List`
- `todo ready` -> `Store.Ready`
- `todo dep add` -> `Store.DepAdd`
- `todo dep tree` -> `Store.DepTree`
