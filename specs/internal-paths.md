# Internal Paths

## Overview
The paths package centralizes default filesystem locations for incrementum state, workspaces, and event logs.

## Defaults
- State: `~/.local/state/incrementum`
- Workspaces: `~/.local/share/incrementum/workspaces`
- Opencode events: `~/.local/share/incrementum/opencode/events`
- Job events: `~/.local/share/incrementum/jobs/events`

## API
- `DefaultStateDir() (string, error)`: returns the default state directory using `os.UserHomeDir`.
- `DefaultWorkspacesDir() (string, error)`: returns the default workspaces directory using `os.UserHomeDir`.
- `DefaultOpencodeEventsDir() (string, error)`: returns the default opencode events directory using `os.UserHomeDir`.
- `DefaultJobEventsDir() (string, error)`: returns the default job events directory using `os.UserHomeDir`.
