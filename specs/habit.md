# Habits

## Overview

Habits are ongoing improvement work without completion state. Unlike regular
todos, habits are never "done" — they represent continuous practices like code
cleanup, performance optimization, or documentation improvement.

Habits work backwards from regular todos: instructions live in version-controlled
files, and when a habit produces a commit, an artifact todo is created to track
the work.

## Instruction Documents

Habit instructions live in `.incrementum/habits/<name>.md` in the repository.

Format:

```markdown
---
models:
  implementation: claude-sonnet-4
  review: claude-haiku
---

# Clean Up

Look for code cleanup opportunities, but do not change CLI behavior.
(Changing how things are printed is fine, but invocations that did
something before should still do that thing after.)

## Guidelines

- Is there duplicated code?
- Is the domain model clear and well-factored?
- Is there needless inconsistency?
...
```

Frontmatter is optional. When present, the `models` section configures which
models to use for implementation and review stages. The body is the prompt
content provided to the agent.

## Artifacts

When a habit produces a commit, an artifact todo is created in the todo store:

- `title`: Summary from the commit message
- `description`: Body from the commit message
- `type`: `task`
- `status`: `done`
- `source`: `habit:<name>`
- `started_at`: Set to creation time
- `completed_at`: Set to creation time

## Commit Message Format

Habit commits use a distinct format that includes the full habit instructions
and the generated commit message:

```
<summary line>

<generated body>

Review comments:

    <review comments if present>

This commit was created as part of the '<name>' habit:

    <habit instructions, indented>
```

The "Review comments" section is only included when the reviewer provided comments
with their ACCEPT verdict.

## Job Integration

### Running a Habit

```
ii job do --habit <name>    # run a specific habit
ii job do --habit           # run the alphabetically first habit
```

### Workflow

1. Load instructions from `.incrementum/habits/<name>.md`
2. Run implementation stage with habit-specific prompt template
3. Run testing stage (same as regular todos)
4. Run step review with habit-specific review template
5. On ACCEPT: commit, create artifact todo, job completes successfully
6. On REQUEST_CHANGES: loop back to implementation with feedback
7. On ABANDON: no artifact created, job completes successfully (nothing worth
   doing right now is a valid outcome)

Habits do not have a project review stage.

### do-all Integration

```
ii job do-all --habits
```

Runs ready todos in priority order first. When the todo queue is empty,
round-robins through habits in alphabetical order.

### Templates

Habit-specific templates:

| File                          | Stage        |
| ----------------------------- | ------------ |
| `prompt-habit-implementation.tmpl` | implementing |
| `prompt-habit-review.tmpl`         | reviewing    |

Templates receive the same data as regular job templates, plus:

- `HabitName` (`string`): Name of the habit (filename without extension)
- `HabitInstructions` (`string`): Full text of the habit instruction document

## Differences from Regular Todos

| Aspect | Regular Todo | Habit |
| ------ | ------------ | ----- |
| Has completion state | Yes | No |
| Instructions live in | Todo store | Git (`.incrementum/habits/`) |
| Project review | Yes | No |
| Priority | Comparable (P0-P4) | Always after all todos |
| ABANDON meaning | Task impossible | Nothing worth doing now |
| Parallel execution | One worker per todo | Multiple workers OK |
| Artifact tracking | Status changes | Creates done todos |

## Package API

The `habit` package provides functions for managing habit files:

| Function | Description |
| -------- | ----------- |
| `Load(repoPath, name)` | Load a habit by name, returns `*Habit` with parsed content |
| `List(repoPath)` | List all habit names, sorted alphabetically |
| `First(repoPath)` | Load the alphabetically first habit, or nil if none |
| `Path(repoPath, name)` | Return the file path for a habit (does not check existence) |
| `Exists(repoPath, name)` | Check if a habit file exists |
| `Create(repoPath, name)` | Create a new habit file with template, returns path |

### Habit Type

```go
type Habit struct {
    Name                string // filename without extension
    Instructions        string // document body (after frontmatter)
    ImplementationModel string // from frontmatter, if present
    ReviewModel         string // from frontmatter, if present
}
```

### Default Template

New habits are created with a template containing:

- A heading with the habit name (dashes/underscores converted to spaces)
- Placeholder description text
- A Guidelines section with placeholder items

## CLI Mapping

The CLI provides subcommands for managing habits:

- `habit list` -> `habit.List`
- `habit show <name>` -> `habit.Path` + reads file directly (to show raw content including frontmatter)
- `habit edit <name>` (`habit update`) -> `habit.Path` + opens `$EDITOR`
- `habit create <name>` -> `habit.Create` + opens `$EDITOR`

### List

```
ii habit list
```

Lists all habit names in alphabetical order. Returns an empty list if no habits
directory exists.

### Show

```
ii habit show <name>
```

Displays the full content of a habit instruction document, including frontmatter
and body.

### Edit

```
ii habit edit <name>
ii habit update <name>
```

Opens the habit file in `$EDITOR`. The file path is
`.incrementum/habits/<name>.md`.

### Create

```
ii habit create <name>
```

Creates a new habit file at `.incrementum/habits/<name>.md` with a template and
opens it in `$EDITOR`. Returns an error if the habit already exists.
