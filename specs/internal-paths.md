# Internal Paths

## Overview
The paths package centralizes default filesystem locations for incrementum state, workspaces, and opencode event logs.

## Defaults
- State: `~/.local/state/incrementum`
- Workspaces: `~/.local/share/incrementum/workspaces`
- Opencode events: `~/.local/share/incrementum/opencode/events`

## API
- `DefaultStateDir() (string, error)`: returns the default state directory using `os.UserHomeDir`.
- `DefaultWorkspacesDir() (string, error)`: returns the default workspaces directory using `os.UserHomeDir`.
- `DefaultOpencodeEventsDir() (string, error)`: returns the default opencode events directory using `os.UserHomeDir`.
