# Internal Paths

## Overview
The paths package centralizes default filesystem locations for incrementum state and workspaces.

## Defaults
- State: `~/.local/state/incrementum`
- Workspaces: `~/.local/share/incrementum/workspaces`

## API
- `DefaultStateDir() (string, error)`: returns the default state directory using `os.UserHomeDir`.
- `DefaultWorkspacesDir() (string, error)`: returns the default workspaces directory using `os.UserHomeDir`.
