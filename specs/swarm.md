# Swarm Subcommand

## Overview

The swarm package coordinates multiple jobs in parallel by running a single
server process and interacting with it through JSON-over-HTTP RPCs. The CLI
subcommand is a thin wrapper around the public package APIs.

## Job Orchestration

- Each job runs in its own workspace.
- Before running a job, the server acquires a workspace at `main` (creating a
  new empty change if `main` is immutable) and runs the job from that workspace
  path.
- When a new change is created for a job, its description is set to `staging for todo <id>`.
- Jobs run within the server process; client commands only stream events or
  send control signals.

## RPCs

RPCs use JSON over HTTP with the following endpoints:

- `POST /do` with `{ "todo_id": "..." }` returns `{ "job_id": "..." }`.
- `POST /kill` with `{ "job_id": "..." }` interrupts the job.
- `POST /tail` with `{ "job_id": "..." }` streams newline-delimited JSON events
  (all events so far, followed by new ones). The server waits for the event log
  to appear and relies on the request context for cancellation.
- `POST /logs` with `{ "job_id": "..." }` returns `{ "events": [...] }` (empty
  list if the event log does not exist yet).
- `POST /list` returns `{ "jobs": [...] }`.

Todo RPCs follow the todo store API shapes:

- `POST /todos/list` with `{ "filter": { ... } }` returns `{ "todos": [...] }` and
  accepts the JSON form of `todo.ListFilter`.
- `POST /todos/create` with `{ "title": "...", "options": { ... } }` returns
  `{ "todo": { ... } }` and accepts the JSON form of `todo.CreateOptions`.
- `POST /todos/update` with `{ "ids": ["..."], "options": { ... } }` returns
  `{ "todos": [...] }` and accepts the JSON form of `todo.UpdateOptions`.

Events are the same `job.Event` JSON objects stored in job event logs.

## Configuration

`swarm` reads `.incrementum/config.toml` from the repo root.

```toml
[swarm]
port = 8088
```

Port precedence is:

1. CLI `--addr` flag.
2. `.incrementum/config.toml` `swarm.port`.
3. Default port `8088`.

## Commands

### `ii swarm serve --addr=`

Start the swarm server for the current repository.

### `ii swarm do [todo-id] [job do flags] [--path=] --addr=`

Create a todo in the provided repo path (or use the current repo), start a job
in the server, and stream job events to stdout. When a todo id is provided, todo
creation flags are not allowed. Interrupts stop streaming but leave the job
running.

### `ii swarm kill <job-id>`

Interrupt a running job in the server.

### `ii swarm tail <job-id>`

Stream job events from the server, including all existing events.

### `ii swarm logs <job-id>`

Print job event logs formatted with the job log formatter.

### `ii swarm list`

List swarm jobs. Defaults to active jobs only; use `--all` or `--status` to
change filters.

Columns: `JOB`, `TODO`, `STAGE`, `STATUS`, `AGE`, `DURATION`, `TITLE`.

### `ii swarm client`

Launch the swarm TUI client for the current repository.

## TUI Client

The swarm TUI is a Bubble Tea client (`ii swarm client`) that connects to the
swarm server and operates on todos and jobs. It uses the same configuration and
`--addr` resolution as other swarm subcommands.

### Layout

- Tab strip across the top with two tabs: `Todo` and `Jobs`.
- Main content splits into a left list pane and a right detail pane.
- The list pane scrolls; the detail pane can scroll independently when
  content exceeds the viewport.
- The default focus is the list pane.

### Global Keys

- `[` / `]`: move between tabs.
- `q` or `ctrl+c`: quit the TUI.

### Pane Focus

- `enter`: move focus from the list pane to the detail pane.
- `esc`: return focus to the list pane.
- When the detail pane is focused, `pgup`/`pgdown` and `home`/`end` scroll the
  pane contents.

### Todo Tab

The list pane shows todos from the swarm server (via `POST /todos/list`) using
the same defaults as `todo.List` (all non-tombstone todos).

List pane actions:

- `c`: create a new todo draft and focus the detail pane for editing.
- `s`: open a confirmation modal to start a swarm job for the selected todo;
  confirming calls `POST /do` and opens the job in the Jobs tab.

Detail pane behavior:

- Editable fields: `title`, `description`, `status`, `priority`, `type`.
- Read-only fields: `id`, `created_at`, `updated_at`, `started_at`,
  `closed_at`, `completed_at`, `deleted_at`, `delete_reason`.
- `tab` / `shift+tab`: move between fields when editing.
- `ctrl+s`: save changes by calling `POST /todos/create` for new todos or
  `POST /todos/update` for existing ones.
- `esc` returns to the list pane; if edits are unsaved, prompt to discard or
  continue editing.

### Jobs Tab

The list pane shows jobs from `POST /list` (active by default). The detail pane
shows the formatted event log for the selected job using the same formatter as
`ii swarm logs`.

- On selection, the client loads existing events via `POST /logs` and, when the
  job is active, streams new events via `POST /tail`.
