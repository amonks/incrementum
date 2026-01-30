# Events

This spec describes the event stream and rendering rules for opencode/job logs.

## Sources

- Opencode emits server-sent events (SSE). Incrementum records the raw JSON payloads in `.sse` files under the opencode events directory.
- Job events are recorded as JSONL entries under the job events directory.

## Observed opencode event types

From the on-disk `.sse` logs in `.local/share/incrementum/opencode/events`, the following event types are present:

- `server.connected`
- `server.heartbeat`
- `session.created`
- `session.updated`
- `session.status`
- `session.idle`
- `session.diff`
- `message.updated`
- `message.part.updated`
- `file.edited`
- `file.watcher.updated`
- `lsp.updated`
- `lsp.client.diagnostics`
- `todo.updated`

## Rendering switches

Each opencode event type has a display switch (see `job/opencode_event_renderer.go`). Default behavior:

- `message.part.updated`: enabled (drives prompt/response/thinking/tool summaries)
- all other listed event types: disabled

Switches control what is shown to users; all events are still recorded in full on disk.

## Text rendering (width-aware)

Only a curated subset of opencode activity is shown in the text logs (CLI/TUI). Output is formatted to the standard line width and indented like other job log entries.

- Tool calls: one-line summaries with start/end markers. Tool start is emitted when the tool begins; tool end is emitted when the state reaches a terminal status (completed, failed, error, cancelled).
  - Example: `Tool start: read file 'src/file.ts'` and `Tool end: read file 'src/file.ts'` (paths are repo-relative when possible).
  - Failed tools show the status: `Tool end: read file '/missing.txt' (failed)`
  - For `apply_patch` tools, file paths are extracted from the unified diff and shown in the summary.
  - For `bash` tools with empty command input, no log is emitted (the command arrives in a subsequent event).
- Prompt text: emitted for `message.part.updated` text parts associated with `role=user` messages.
  - Label: `Opencode prompt:`
- Assistant responses: emitted when an assistant message completes.
  - Label: `Opencode response:`
- Assistant thinking: emitted when an assistant message completes and a reasoning part has non-empty text.
  - Label: `Opencode thinking:`
- Prompt, response, and thinking bodies are rendered as markdown via `internal/markdown` (glamour) before indentation.

## Raw event display

Raw JSON payloads are not rendered by default. If an opencode event payload cannot be decoded into a known shape, it falls back to a generic “Opencode event” block to avoid hiding malformed data in logs.
