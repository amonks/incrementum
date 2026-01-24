# Opencode Subcommand

## Overview

The opencode subcommand integrates the Opencode agent workflow into `ii`.
It wraps the external `opencode` CLI and tracks sessions in the shared
state store, scoped to the current repo slug.

## Package

The public API lives in `opencode/`. It owns session types and state
operations, plus command helpers (`Run`, `Logs`, `Tail`, `Kill`) that are
invoked by the CLI and job workflows.

## Storage

- Session state is stored in `~/.local/state/incrementum/state.json` alongside
  workspace state.
- Opencode session data lives under `~/.local/share/opencode/storage`.
- Event logs streamed from opencode live under `~/.local/share/incrementum/opencode/events`.
- Session metadata is read from `storage/session/<project-id>/<session-id>.json`.
- Prose-only transcripts are reconstructed from `storage/message/<session-id>/`
  and `storage/part/<message-id>/`.
- Opencode session metadata uses the same repo slug naming rules as the
  workspace pool.
- All state updates are serialized using the existing state file lock.

## State Model

Opencode state is stored in the shared state file alongside workspace
state. It adds one top-level collection:

- `opencode_sessions`: map of `repo-slug/session-id` to session info.

### Session

Fields (JSON keys):

- `id`: opencode session id (for example `ses_...`).
- `repo`: repo slug.
- `status`: `active`, `completed`, `failed`, or `killed`.
- `prompt`: full prompt string that was provided to `run`.
- `created_at`: timestamp.
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `completed_at`: timestamp (set when completed/failed/killed).
- `exit_code`: integer exit code for `run`.
- `duration_seconds`: duration in seconds.

## Rules

- Opencode sessions are tracked per repo slug.
- Opencode commands resolve the jj repo root from the current working directory
  when possible (workspace roots map to their source repo). When no repo is
  found, they fall back to the current working directory.
- Session IDs accept case-insensitive prefix matching; prefixes must be
  unambiguous.
- `run` starts `opencode serve` and streams events from `/event` before invoking
  `opencode run --attach=<server-url>`.
- `run` updates session status and exit code when the command exits.
- `kill` records status `killed` and sets `exit_code` to the signal exit code
  reported by opencode when available.
- Logs are read from incrementum's stored event stream and retained
  indefinitely by incrementum.

## Commands

### `ii opencode run [prompt]`

- Prompt is read from stdin when no prompt argument is provided.
- Starts `opencode serve`, opens the event stream, then executes
  `opencode run --attach=<server-url> <prompt>` from the repo root.
- Streams opencode events to `~/.local/share/incrementum/opencode/events`.
- Returns an event channel to callers so they can read the full event stream.
- Creates a new opencode session record in state shortly after the run starts (once opencode writes session metadata).
- Updates status, exit code, and duration when the run finishes.
- Exits with the same code as the opencode run.

### `ii opencode logs <session-id>`

- Resolves the opencode session by id in the current repo.
- Prints a snapshot of the stored event stream to stdout.

### `ii opencode list [--json] [--all]`

- Lists opencode sessions for the current repo.
- Default output is a table matching other list commands.
- Default output includes only active sessions unless `--all` is provided.
- When the list is empty but sessions exist, print a hint to use `--all`.
- Suggested columns: `SESSION`, `STATUS`, `AGE`, `DURATION`, `PROMPT`, `EXIT`.
- `SESSION` highlights the shortest unique prefix across all sessions in the
  repo when ANSI output is enabled.
- `PROMPT` shows only the first line of the prompt; full prompt remains in state.
- `PROMPT` displays `-` when the first line is empty or whitespace-only.
- `PROMPT` header and cells are truncated to the computed prompt column width to keep the table within the viewport.
- `AGE` shows a compact duration in `s`, `m`, `h`, or `d` units.
- `AGE` is `-` when the session is missing timing data.
- `AGE` uses `now - created_at`.
- `DURATION` shows a compact duration in `s`, `m`, `h`, or `d` units.
- `DURATION` is `-` when the session is missing timing data.
- `DURATION` uses `now - created_at` for active sessions, otherwise `updated_at - created_at`.

### `ii opencode kill <session-id>`

- Resolves the opencode session by id in the current repo.
- Sends a termination request via `opencode`.
- Marks the session as `killed` and records exit code/metadata.
