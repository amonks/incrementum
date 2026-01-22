# Opencode Subcommand

## Overview

The opencode subcommand integrates the Opencode agent workflow into `ii`.
It wraps the external `opencode` CLI and tracks long-running sessions in the
shared state store. The server is intended as a global process (per-user) while
session state is scoped to a repo slug.

## Storage

- Session state is stored in `~/.local/state/incrementum/state.json` alongside
  workspace/session state.
- Opencode session logs are stored under
  `~/.local/share/incrementum/opencode/<repo-slug>/<session-id>.log`.
- Opencode session metadata uses the same repo slug naming rules as the
  workspace pool.
- All state updates are serialized using the existing state file lock.

## State Model

Opencode state is stored in the shared state file alongside workspace/session
state. It adds two top-level collections:

- `opencode_daemons`: map of `repo-slug` to daemon info.
- `opencode_sessions`: map of `repo-slug/session-id` to session info.

### Daemon

Fields (JSON keys):

- `repo`: repo slug.
- `status`: `running` or `stopped`.
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `pid`: server process id when known.
- `host`: hostname or socket location used by `opencode serve`.
- `port`: port used by `opencode serve` when applicable.
- `log_path`: absolute path to the daemon log file when captured.

### Session

Fields (JSON keys):

- `id`: session id (hash prefix of prompt + timestamp).
- `repo`: repo slug.
- `status`: `active`, `completed`, `failed`, or `killed`.
- `prompt`: full prompt string that was provided to `run`.
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `completed_at`: timestamp (set when completed/failed/killed).
- `exit_code`: integer exit code for `run`/`wait`.
- `duration_seconds`: duration in seconds.
- `log_path`: absolute path to the log file.

## Rules

- Opencode sessions are tracked per repo slug.
- The daemon is a foreground process invoked via `opencode serve`.
- `ii opencode serve` records daemon state when the process starts and clears
  it when the process exits.
- Daemon metadata comes from explicit `ii opencode serve` flags; if host/port
  are not provided, they are left empty and the daemon is still considered
  running.
- Session IDs accept case-insensitive prefix matching; prefixes must be
  unambiguous.
- When listing or validating daemon state, verify the stored pid is still
  running; if the process is gone, mark status `stopped`.
- `run` always attaches to the daemon; it fails when the daemon is not running.
- `run` returns immediately after starting a session.
- `kill` records status `killed` and sets `exit_code` to the signal exit code
  reported by opencode when available.
- Log files always capture stdout and stderr for opencode runs.
- Logs are retained indefinitely.

## Commands

### `ii opencode serve`

- Start the opencode server by executing `opencode serve`.
- Runs in the foreground and streams logs to the terminal.
- Writes daemon logs to `~/.local/share/incrementum/opencode/<repo-slug>/daemon.log`.
- Records daemon info (pid/host/port) in state while running.
- Does not create any opencode session records.

### `ii opencode run [--attach] [prompt]`

- Prompt is read from stdin when no prompt argument is provided.
- Always attaches to the running daemon and errors if the daemon is not running.
- Creates a new opencode session record with status `active`.
- Creates a log file at `~/.local/share/incrementum/opencode/<repo-slug>/<session-id>.log`.
- Executes `opencode run --attach` and tees stdout/stderr to the log file.
- Uses only the provided prompt/flags; no implicit config overrides.
- Prints the session id only.
- Returns immediately after the opencode session is created.

### `ii opencode logs <session-id>`

- Resolves the opencode session by id in the current repo.
- Prints a snapshot of the log contents to stdout.

### `ii opencode tail <session-id>`

- Resolves the opencode session by id in the current repo.
- Streams live log output (similar to `tail -f`).

### `ii opencode wait <session-id>`

- Resolves the opencode session by id in the current repo.
- Polls `opencode session list --format json` until the session is no longer
  active (or the command reports status/exit data for the session).
- Updates status and exit code in state from the polled opencode metadata when
  available.
- Exits with the same code as the opencode session when provided; otherwise
  exits 0 after marking completion.
- Exits with the same code as the opencode session.

### `ii opencode list [--json] [--all]`

- Lists opencode sessions for the current repo.
- Default output is a table matching other list commands.
- Default output includes only active sessions unless `--all` is provided.
- Suggested columns: `SESSION`, `STATUS`, `AGE`, `PROMPT`, `EXIT`.
- `SESSION` highlights the shortest unique prefix across all sessions in the
  repo when ANSI output is enabled.
- `PROMPT` shows only the first line of the prompt; full prompt remains in state.
- `PROMPT` displays `-` when the first line is empty or whitespace-only.
- `AGE` shows a compact duration in `s`, `m`, `h`, or `d` units.
- `AGE` is `-` when the session is missing timing data.
- `AGE` prefers `duration_seconds`, otherwise uses `completed_at - started_at` for finished sessions or `now - started_at` for active sessions.

### `ii opencode kill <session-id>`

- Resolves the opencode session by id in the current repo.
- Sends a termination request via `opencode`.
- Marks the session as `killed` and records exit code/metadata.
