# Swarm Subcommand

## Overview

The swarm package coordinates multiple jobs in parallel by running a single
server process and interacting with it through JSON-over-HTTP RPCs. The CLI
subcommand is a thin wrapper around the public package APIs.

The swarm server logs request failures and job lifecycle events (start,
completion, failure, and panic recovery) to stderr with a `swarm:` prefix so
operators can troubleshoot unexpected job outcomes. Job panics log stack traces
alongside a failure summary. If an RPC handler panics, the server logs the stack
trace and returns a 500 error instead of crashing.

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
- `POST /list` with `{ "filter": { ... } }` returns `{ "jobs": [...] }` and
  accepts the JSON form of `job.ListFilter`.
- Requests that omit `todo_id` or `job_id` return a 400 error with a message
  describing the missing field.
- Unexpected RPC panics return a 500 response with `{"error": "internal server error"}`.

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

On startup, the command logs the full address (for example, `Swarm server
listening on 127.0.0.1:8088`).

`--agent` selects the opencode agent for jobs run by the server and overrides
`INCREMENTUM_OPENCODE_AGENT`.

The server writes operational logs to stderr for request errors, job start/stop
events, handler panics, and workspace cleanup failures.

### `ii swarm do [todo-id] [job do flags] [--path=] --addr=`

Create a todo in the provided repo path (or use the current repo), start a job
in the server, and stream job events to stdout. When a todo id is provided, todo
creation flags are not allowed. Interrupts stop streaming but leave the job
running.

### `ii swarm kill <job-id>`

Interrupt a running job in the server.

### `ii swarm tail <job-id|todo-id>`

Stream job events from the server, including all existing events. When given a
todo ID, the server selects the most recent active job for that todo (or the
latest job if none are active).

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

- Tab strip across the top with two tabs: `[1] Todo` and `[2] Jobs`, with a help hint.
- A help bar sits below the tab strip and lists the relevant keyboard shortcuts for the focused pane.
- Main content splits into a left list pane and a right detail pane.
- The list pane scrolls; the detail pane can scroll independently when
  content exceeds the viewport.
- The default focus is the list pane.

### Global Keys

- `[` / `]`: move between tabs.
- `1` / `2`: jump directly to the Todo or Jobs tab.
- `tab` / `shift+tab`: cycle tabs when the list pane is focused.
- `q` or `ctrl+c`: quit the TUI.
- `?`: toggle the help overlay.

The help overlay lists the available shortcuts and closes with `?` or `esc`.

### Pane Focus

- `enter`: move focus from the list pane to the detail pane.
- `esc`: return focus to the list pane.
- `up` / `down` or `j` / `k`: move the selection in the list pane.
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

## Web Client

`ii swarm serve` also serves a no-JavaScript web client over the same HTTP
server. The web client mirrors the TUI affordances using HTML forms and server-
side templates. It is implemented in the `web` package using only the standard
library.

### Web Runtime Model

- Web routes are mounted under `/web` to avoid conflicts with the swarm RPC
  endpoints.
- The web handler talks to the swarm RPCs over HTTP (even when embedded in the
  same process) using the root RPC paths.
- On startup, the web server fetches initial todos and jobs to seed in-memory
  state.
- Each request refreshes the relevant state by calling the swarm RPCs before
  rendering a page. The handler may reuse cached data from the last refresh to
  avoid redundant calls within a request.
- Job event details are fetched via `POST /logs` on demand. The web client does
  not use streaming (`POST /tail`); active jobs include a manual refresh action.

### Routes

- `GET /` redirects to `/web/todos`.
- `GET /web/todos` renders the Todo tab UI.
- `POST /web/todos/create` creates a new todo via `POST /todos/create`.
- `POST /web/todos/update` updates a todo via `POST /todos/update`.
- `POST /web/jobs/start` starts a job for a todo via `POST /do` and redirects to
  the Jobs view with the new job selected.
- `GET /web/jobs` renders the Jobs tab UI.
- `POST /web/jobs/kill` interrupts a job via `POST /kill`.
- `POST /web/jobs/refresh` reloads job detail events via `POST /logs` and
  redirects back to the Jobs view.

### Layout

- Page layout matches the TUI: a tab strip (`Todos`, `Jobs`) across the top and
  two panes below.
- The left pane lists items; the right pane shows details for the selected item.
- The list selection is controlled by a query parameter (`/web/todos?id=...`,
  `/web/jobs?id=...`) and defaults to the first item.
- The todos view supports `?create=1` to show a blank detail form without
  selecting an existing todo.
- The detail pane uses HTML forms for editing or triggering actions.
- The UI uses minimal CSS included in the HTML template; no external assets or
  JavaScript are required.

### Todo View

- The list pane shows todos from `POST /todos/list` using the same defaults as
  `todo.List` (all non-tombstone todos).
- A "Create" action renders a blank detail form for a new todo.
- Detail fields and read-only fields match the TUI (including id and timestamps).
- Saving uses the same behavior as the TUI: new todos call
  `POST /todos/create`, existing todos call `POST /todos/update`.
- A "Start job" action confirms intent and calls `POST /do`.

#### Todo Detail Forms

- Editable fields are submitted with the same names as the todo JSON keys.
- `status`, `priority`, and `type` are rendered as `<select>` inputs with the
  same allowed values as the todo store.
- `description` is rendered as a multi-line `<textarea>`.
- Read-only fields render as plain text and are omitted from submissions.
- The create form defaults to the same values as `todo create`.
- Form submissions redirect back to `/web/todos` with the selected todo id (or
  `?create=1` when creation fails).
- Errors returned from the RPCs render an inline error message in the detail
  pane and preserve any user-entered field values.

### Jobs View

- The list pane shows jobs from `POST /list` (active by default).
- The detail pane shows the formatted job event log using the same formatter as
  `ii swarm logs`.
- Active jobs show a "Refresh" action that re-fetches events via `POST /logs`.
- A "Kill job" action interrupts the job via `POST /kill`.
- Job form actions redirect back to `/web/jobs` with the selected job id.
- Refresh failures surface as an inline error in the detail pane after the
  redirect, matching other RPC error handling in the web client.
