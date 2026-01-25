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
