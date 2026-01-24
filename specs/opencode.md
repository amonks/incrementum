# Opencode Subcommand

## Overview

The opencode subcommand integrates the Opencode agent workflow into `ii`.
It wraps the external `opencode` CLI and tracks sessions in the shared
state store, scoped to the current repo slug.

## Storage

- Session state is stored in `~/.local/state/incrementum/state.json` alongside
  workspace state.
- Opencode session data lives under `~/.local/share/opencode/storage`.
- Session metadata is read from `storage/session/<project-id>/<session-id>.json`.
- Session logs are reconstructed from `storage/message/<session-id>/` and
  `storage/part/<message-id>/`.
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
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `completed_at`: timestamp (set when completed/failed/killed).
- `exit_code`: integer exit code for `run`.
- `duration_seconds`: duration in seconds.

## Rules

- Opencode sessions are tracked per repo slug.
- Opencode commands use the current working directory as the repo path; they do
  not resolve jj workspace roots or session mappings.
- Session IDs accept case-insensitive prefix matching; prefixes must be
  unambiguous.
- `run` executes `opencode run` directly and blocks until completion.
- `run` updates session status and exit code when the command exits.
- `kill` records status `killed` and sets `exit_code` to the signal exit code
  reported by opencode when available.
- Logs are read from opencode storage and retained indefinitely by opencode.

## Commands

### `ii opencode run [prompt]`

- Prompt is read from stdin when no prompt argument is provided.
- Executes `opencode run <prompt>` from the repo root and streams output.
- Creates a new opencode session record in state after the run completes.
- Updates status, exit code, and duration when the run finishes.
- Exits with the same code as the opencode run.

### `ii opencode logs <session-id>`

- Resolves the opencode session by id in the current repo.
- Prints a snapshot of the reconstructed log contents to stdout.

### `ii opencode tail <session-id>`

- Resolves the opencode session by id in the current repo.
- Streams log output by polling opencode storage (similar to `tail -f`).

### `ii opencode list [--json] [--all]`

- Lists opencode sessions for the current repo.
- Default output is a table matching other list commands.
- Default output includes only active sessions unless `--all` is provided.
- When the list is empty but sessions exist, print a hint to use `--all`.
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
