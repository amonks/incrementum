# Internal Editor

## Overview
The editor package wraps `$EDITOR` and provides an interactive flow for editing todos.

## Editor Helpers
- `IsInteractive` reports whether stdin is a terminal.
- `Edit` launches `$EDITOR` (defaults to `vi`) and waits for exit.

## Todo Editing
- `TodoData` models the fields used in the editable TOML template.
- `RenderTodoTOML` renders a TOML header and description body separated by `---`.
- `ParseTodoTOML` validates the TOML output and normalizes type/status fields.
- `EditTodo` and `EditTodoWithData` create a temp file, launch the editor, and parse the result.
- `ParsedTodo` converts into `todo.CreateOptions` or `todo.UpdateOptions` for persistence.
- The todo template always includes a `status` field; create defaults to `open` unless overridden by the caller.
