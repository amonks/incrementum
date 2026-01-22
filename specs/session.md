# Session Subcommand

## Overview

The session subcommand coordinates todo status changes with workspace
acquisition/release. Session state is persisted alongside workspace state in the
shared state file so multiple processes can coordinate safely.

## Storage

- Session state is stored in `~/.local/state/incrementum/state.json`.
- Session records are scoped per repo using the same repo slug as workspace
  state.
- Session records store the workspace name only; workspace paths are resolved
  from workspace state when needed.

## Session Model

Fields (JSON keys):

- `id`: session id (hash prefix of todo id + timestamp).
- `repo`: repo slug.
- `todo_id`: full resolved todo id.
- `workspace_name`: `ws-###`.
- `status`: `active`, `completed`, or `failed`.
- `topic`: human-readable summary (from `--topic` or argv string).
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `completed_at`: timestamp (set when completed/failed).
- `exit_code`: integer exit code for `run`.
- `duration_seconds`: duration in seconds for `run` (or when `done`/`fail` is
  invoked after `start`).

## Rules

- Only one active session per todo.
- `start` errors if the todo status is `in_progress`, `done`, `closed`, or
  `tombstone`.
- `start` sets todo status to `in_progress`.
- `done` sets todo status to `done`.
- `fail` sets todo status to `open`.
- Session records own the workspace lease; `done`/`fail` release the workspace
  recorded in the session.
- Resolution for `done`/`fail`:
  1) if a todo id is provided, resolve the session by todo id
  2) else if the cwd is a workspace, resolve the session by workspace
  3) otherwise error
- Todo id matching for sessions uses case-insensitive prefix matching against
  active sessions in the repo.

## Commands

### `ii session start [todo-id] [--topic <text>]`

When a todo id is provided, start a session for the existing todo.
When no id is provided, create a new todo using the same flags as `ii todo create`
(`--title`, `--type`, `--priority`, `--description/--desc`, `--deps`, `--edit`, `--no-edit`).

- Reject combining a todo id with todo creation flags.
- For new todos, default to opening $EDITOR when running interactively and no create flags are provided.
- Resolve todo id.
- Error if todo status is `in_progress`, `done`, `closed`, or `tombstone`.
- Acquire a workspace.
- Update todo status to `in_progress`.
- Create a session with status `active`.
- If updating the todo or creating the session fails, release the workspace (and reset todo status if needed).
- Print the workspace path.

### `ii session done [todo-id]`

- Resolve the session using the rules above.
- Release the workspace referenced by the session.
- Update todo status to `done`.
- Mark the session as `completed` and record `completed_at`.

### `ii session fail [todo-id]`

- Resolve the session using the rules above.
- Release the workspace referenced by the session.
- Update todo status to `open`.
- Mark the session as `failed` and record `completed_at`.

### `ii session run <todo-id> -- <cmd> [args...]`

- Standard passthrough after `--`.
- Acquire a workspace and create a session (same as `start`).
- Set `topic` to the argv string.
- Run the command in the workspace.
- If exit code is 0:
  - mark the todo as `done`
  - mark the session `completed`
- If exit code is nonzero:
  - mark the todo as `open`
  - mark the session `failed`
- Always release the workspace.
- Record exit code and duration seconds.

### `ii session list [--status <status>] [--all] [--json]`

- Default output is a table matching other list commands.
- Default behavior lists only `active` sessions unless `--status` or `--all` is provided.
- `--status` filters to `active`, `completed`, or `failed`.
- `--all` lists all statuses.
- `--json` emits structured output.
- Suggested columns: `SESSION`, `TODO`, `STATUS`, `WORKSPACE`, `AGE`, `TOPIC`, `EXIT`.
- `AGE` shows a compact duration in `s`, `m`, `h`, or `d` units.
- Todo IDs use the same shortest-unique prefix lengths as todo list output.
