# Job Subcommand

## Overview

The job package and subcommand automate todo completion via opencode. A job
runs from the current working directory, invokes opencode to implement the
todo, runs acceptance tests, runs opencode to review changes, generates a commit
message, and describes the commit. Jobs retry on test failure or review
rejection until opencode decides to abandon.

## Architecture

The job implementation lives in a Go package with clean exports; the `cmd/ii`
subcommand stays a thin wrapper that wires flags and delegates to the package.

## Testing

Follow our usual testing practice:

- Prefer lots of focused unit tests in `job/`.
- Add a handful of end-to-end testscript tests in `cmd/ii`.

## Storage

- Job state stored in `~/.local/state/incrementum/state.json` alongside other
  state.
- Jobs are scoped per repo using the same repo slug as other state.
- Jobs do not create sessions or workspaces.
- Job records track opencode sessions created during the job.

## Job Model

Fields (JSON keys):

- `id`: 8-character job id (hash of todo_id + timestamp).
- `repo`: repo slug.
- `todo_id`: full resolved todo id.
- `stage`: `implementing`, `testing`, `reviewing`, `committing`.
- `feedback`: feedback from last failed stage (test results table or review
  feedback).
- `opencode_sessions`: list of `{"purpose": string, "id": string}` tracking
  opencode sessions created during this job.
- `status`: `active`, `completed`, `failed`, `abandoned`.
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `completed_at`: timestamp.

## Feedback File

Opencode communicates review outcomes by writing to `.incrementum-feedback` in the
job working directory (the repo root). If the file is missing there, fall back to
the repo root (for backwards compatibility).

Format:

```
<OUTCOME>

<optional details>
```

Where `<OUTCOME>` (first line, trimmed) is one of:

- `ACCEPT` - changes look good, proceed.
- `ABANDON` - task is impossible or misguided, give up.
- `REQUEST_CHANGES` - followed by blank line and feedback text.

If the file doesn't exist after review, treat as `ACCEPT`.

## Commit Message File

Opencode writes the generated commit message to `.incrementum-commit-message` in the
job working directory (the repo root). If the file is missing there, fall back to
the repo root (for backwards compatibility).

## State Machine

```
implementing -> testing -> reviewing -> committing -> completed
     ^             |            |
     |             |            |
     +-------------+------------+
       (test failure or REQUEST_CHANGES)

reviewing -> abandoned (ABANDON)
any stage -> failed (unrecoverable error)
```

### implementing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. Delete `.incrementum-feedback` if it exists.
3. Run opencode with `implement.tmpl` prompt from the repo root (PWD set to the repo root).
4. Template receives: `Todo`, `Feedback` (empty string on initial run).
5. Record opencode session in `opencode_sessions` with purpose `implement`.
6. Run opencode to completion.
7. If opencode fails (nonzero exit): mark job `failed`.
8. Transition to `testing`.

### testing

1. Run each test command from config sequentially.
2. Capture exit code for each command.
3. If any command fails (nonzero exit):
   - Build feedback as markdown table with columns `Command` and `Exit Code`.
   - Transition to `implementing`.
4. If all pass: transition to `reviewing`.

### reviewing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. Delete `.incrementum-feedback` if it exists.
3. Run opencode with `review.tmpl` prompt from the repo root (PWD set to the repo root).
4. Template receives: `Todo`.
5. Template instructs opencode to inspect changes (e.g., `jj diff`) and write
   outcome to `.incrementum-feedback`.
6. Record opencode session in `opencode_sessions` with purpose `review`.
7. Run opencode to completion.
8. If opencode fails (nonzero exit): mark job `failed`.
9. Read `.incrementum-feedback`:
   - Delete `.incrementum-feedback` after reading.
   - Missing or first line is `ACCEPT`: transition to `committing`.
   - First line is `ABANDON`: mark job `abandoned`.
    - First line is `REQUEST_CHANGES`: extract feedback (lines after first blank
      line), transition to `implementing`.
   - Other first line: treat as invalid format, mark job `failed`.

### committing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. Delete `.incrementum-commit-message` if it exists.
3. Run opencode with `commit-message.tmpl` prompt from the repo root (PWD set to the repo root).
4. Template receives: `Todo`.
5. Template instructs opencode to generate commit message and write to
   `.incrementum-commit-message`.
6. Record opencode session in `opencode_sessions` with purpose `commit-message`.
7. Run opencode to completion.
8. If opencode fails (nonzero exit): mark job `failed`.
9. Read `.incrementum-commit-message`.
10. Delete `.incrementum-commit-message` after reading.
11. Format final message using `commit.tmpl` with: `Todo`, `Message` (from file).
12. Best-effort `jj workspace update-stale` in the repo working directory.
13. Run `jj describe -m "<formatted message>"` in the repo working directory.
14. If describe fails: mark job `failed`.
15. Mark job `completed`.

## Failure Handling

- `failed`: unrecoverable error (describe fails, invalid feedback format).
- `abandoned`: opencode decided the task is impossible.

Both reopen the todo.

On interrupt (SIGINT), mark job `failed` and reopen the todo.

## Todo Status Updates

- Before running, mark the todo `in_progress`.
- When a job completes successfully, mark the todo `done`.
- When a job fails or is abandoned, reopen the todo (`open`).

## Config

```toml
[job]
test-commands = [
  "go test ./...",
  "golangci-lint run",
]
```

## Templates

Bundled defaults via `//go:embed`, overridable by placing files in
`.incrementum/prompts/`.

| File                  | Stage        | Variables                     |
| --------------------- | ------------ | ----------------------------- |
| `implement.tmpl`      | implementing | `Todo`, `Feedback`, `WorkspacePath` |
| `review.tmpl`         | reviewing    | `Todo`, `WorkspacePath`                        |
| `commit-message.tmpl` | committing   | `Todo`, `WorkspacePath`                        |
| `commit.tmpl`         | committing   | `Todo`, `Message`, `WorkspacePath`             |

Templates use Go `text/template` syntax.

`Todo` exposes: `ID`, `Title`, `Description`, `Type`, `Priority`.
`WorkspacePath` is the absolute path to the job's workspace root.

## Commands

### `ii job do [todo-id | creation-flags]`

Create and run a job to completion (blocking).

- If todo-id provided: use existing todo.
- If creation flags provided: create todo first (same flags as `ii todo create`:
  `--title`, `--type`, `--priority`, `--description/--desc`, `--deps`,
  `--edit/--no-edit`).
- If no args and interactive: open $EDITOR to create todo.
- If `--rev` is omitted, default to `trunk()`.

Behavior:

1. Resolve or create todo.
2. Release the todo store workspace once the todo is loaded.
3. Mark the todo `in_progress`.
4. Run the job from the repo root (no session/workspace or new change is created).
5. Output job context: workdir and full todo details.
6. Create job record with status `active`, stage `implementing`.
7. Run state machine to completion.
8. Output progress: stage transitions.
9. On success: mark todo done and print final commit info.
10. On failure/abandon: reopen todo and print reason.

Exit codes:

- 0: completed.
- 1: failed or abandoned.

### `ii job list [--status <s>] [--all] [--json]`

List jobs for current repo.

- Default: active jobs only.
- `--status`: filter by status (case-insensitive).
- `--all`: show all statuses.
- `--json`: structured output.

Columns: `JOB`, `TODO`, `STAGE`, `STATUS`, `AGE`.

`JOB` highlights the shortest unique prefix across all jobs in the repo.

`TODO` uses the same prefix highlighting as other todo output.

`SESSION` uses the shortest unique prefix across job session IDs in the repo.

When list is empty but jobs exist, print hint about `--all`.

### `ii job show <job-id>`

Show detailed job info.

Output includes:

- Job ID, status, stage.
- Todo ID and title.
- Feedback (if any).
- Opencode sessions with purposes.

### `ii job logs <job-id>`

Show aggregated logs from all opencode sessions in the job.

Concatenates logs in chronological order, with headers indicating purpose and
session ID.
