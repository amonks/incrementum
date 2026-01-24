# Job Subcommand

## Overview

The job package and subcommand automate todo completion via opencode. A job
runs from the current working directory and executes a work loop: opencode is
asked to complete the next highest-priority step and write a commit message,
tests run, opencode reviews the change, and the result is committed. The loop
continues until opencode makes no changes, then the job runs tests and a final
project review before completing. Jobs retry on test failure or review rejection
until opencode decides to abandon.

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
- Job event logs are stored as JSONL at
  `~/.local/share/incrementum/jobs/events/<job-id>.jsonl`.
- Job event entries use opencode's event shape (`id`, `name`, `data`) and include
  both opencode events and job-specific events (stage changes, prompts, opencode
  transcripts, test results, review feedback, commit messages, opencode session
  boundaries).

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
- `created_at`: timestamp.
- `started_at`: timestamp.
- `updated_at`: timestamp.
- `completed_at`: timestamp.

## Feedback File

Opencode communicates review outcomes by writing to `.incrementum-feedback` in the
job workspace root (`WorkspacePath`).

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
job workspace root (`WorkspacePath`) during the implementing stage.

## State Machine

```
implementing -> testing -> reviewing -> committing -> implementing
     ^             |            |           |
     |             |            |           +-> (continue work loop)
     |             |            +--------------> implementing (REQUEST_CHANGES)
     |             +--------------------------> implementing (test failure)
     |
     +-> (no changes) -> testing -> reviewing -> completed

reviewing -> abandoned (ABANDON)
any stage -> failed (unrecoverable error)
```

### implementing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. Delete `.incrementum-feedback` from the workspace root if it exists.
3. Record the current working copy commit id.
4. Run opencode with `prompt-implementation.tmpl` when no feedback is present,
   or `prompt-feedback.tmpl` when responding to feedback (PWD set to the
   workspace root).
5. Template receives: `Todo`, `Feedback`, and `Message` (previous commit message
   when responding to feedback).
6. Record opencode session in `opencode_sessions` with purpose `implement`.
7. Run opencode to completion.
8. If opencode fails (nonzero exit): mark job `failed`.
9. Record the current working copy commit id again.
10. If the commit id did not change:
    - Delete `.incrementum-commit-message` from the workspace root if it exists.
   - Flag the next testing/review cycle as the final project review.
11. If the commit id changed:
    - Read `.incrementum-commit-message` from the workspace root.
    - Store the message for the committing stage.
12. Transition to `testing`.

### testing

1. Run each test command from config sequentially.
2. Capture exit code for each command.
3. If any command fails (nonzero exit):
   - Build feedback as markdown table with columns `Command` and `Exit Code`.
   - Transition to `implementing`.
4. If all pass: transition to `reviewing`.
5. If the job was in final project review when tests failed, the next implementing
   stage restarts the work loop.

### reviewing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. Delete `.incrementum-feedback` from the workspace root if it exists.
3. Run opencode with:
   - `prompt-commit-review.tmpl` during the work loop, or
   - `prompt-project-review.tmpl` during the final project review.
4. Template receives: `Todo`, `Message` (commit message from the implementing stage).
   If the review template does not reference `Message`, the job appends a
   `<commit_message>` block with the message before rendering.
   - If the commit message is required for the step review and missing, fail with
     a descriptive error that calls out the opencode implementation prompt and
     expected `.incrementum-commit-message` location.
5. Template instructs opencode to inspect changes (or the commit sequence for
   project review) and write outcome to `.incrementum-feedback`.
6. Record opencode session in `opencode_sessions` with purpose `review` or
   `project-review`.
7. Run opencode to completion.
8. If opencode fails (nonzero exit): mark job `failed`.
9. Read `.incrementum-feedback` from the workspace root:
   - Delete `.incrementum-feedback` after reading.
   - Missing or first line is `ACCEPT`:
     - During the work loop: transition to `committing`.
     - During project review: mark job `completed`.
   - First line is `ABANDON`: mark job `abandoned`.
   - First line is `REQUEST_CHANGES`: extract feedback (lines after first blank
     line), transition to `implementing` and restart the work loop if needed.
   - Other first line: treat as invalid format, mark job `failed`.

### committing

1. Best-effort `jj workspace update-stale` in the repo working directory.
2. Format final message with a fixed commit message layout (not templated). The
   format uses the opencode-generated summary/body plus a todo block, reflowed to
   80/76/72 columns with 0/4/8-space indentation.
3. Best-effort `jj workspace update-stale` in the repo working directory.
4. Run `jj commit -m "<formatted message>"` in the repo working directory.
5. If commit fails: mark job `failed`.
6. Transition back to `implementing` to continue the work loop.

Commit message format:

```
<summary line>

Here is a generated commit message:

    <reflowed body>

This commit is a step towards implementing this todo:

    ID: <id>
    Title: <title>
    Type: <type>
    Priority: <priority> (<name>)
    Description:
        <reflowed description>
```

## Failure Handling

- `failed`: unrecoverable error (commit fails, invalid feedback format).
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
`.incrementum/templates/`.

| File                   | Stage        | Variables                             |
| ---------------------- | ------------ | ------------------------------------- |
| `prompt-implementation.tmpl` | implementing | `Todo`, `Feedback`, `Message`, `CommitLog`, `WorkspacePath`, `ReviewInstructions`, `TodoBlock`   |
| `prompt-feedback.tmpl`       | implementing | `Todo`, `Feedback`, `Message`, `CommitLog`, `WorkspacePath`, `ReviewInstructions`, `TodoBlock`   |
| `prompt-commit-review.tmpl`  | reviewing    | `Todo`, `Message`, `CommitLog`, `WorkspacePath`, `ReviewInstructions`, `TodoBlock`    |
| `prompt-project-review.tmpl` | reviewing    | `Todo`, `CommitLog`, `WorkspacePath`, `ReviewInstructions`, `TodoBlock`               |

Templates use Go `text/template` syntax (commit messages are generated in code).

`Todo` exposes: `ID`, `Title`, `Description`, `Type`, `Priority`, `Status`,
`CreatedAt`, `UpdatedAt`, `ClosedAt`, `DeletedAt`, `DeleteReason`.
`CommitLog` is the list of commits recorded so far with fields `ID` and
`Message`.
`WorkspacePath` is the absolute path to the job's workspace root.
`ReviewInstructions` is the standard review output instructions block.
`TodoBlock` is a formatted `<todo>` block that includes ID, title, type,
priority, and description.

The prompt renderer provides a `review_questions` template definition with the
default review question list.

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
4. Run the job from the workspace root (no session/workspace or new change is created).
5. Output job context: workdir and full todo details.
6. Create job record with status `active`, stage `implementing`.
7. Run state machine to completion.
8. Output progress: stage transitions and formatted logs (prompts, opencode
   transcripts, commit messages, test results, review feedback) with 80-column
   reflow and 0/4/8-space indentation for document hierarchy.
9. On success: mark todo done and print final commit info with 80-column reflow
   and 0/4/8-space indentation.
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

Columns: `JOB`, `TODO`, `STAGE`, `STATUS`, `AGE`, `DURATION`.

`AGE` uses `now - created_at`.

`DURATION` uses `now - created_at` for active jobs, otherwise
`updated_at - created_at`.

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

Show the combined job event stream.

Reads the job's JSONL event log and prints entries in the order they were
recorded, formatting stage transitions and logs with the same 80-column reflow
and 0/4/8-space indentation used during `ii job do` output.
