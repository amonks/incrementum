# Internal Config

## Overview
The config package loads project and global `incrementum.toml` configuration files and runs hook scripts.

## Configuration Model
- `Config` holds workspace and job configuration.
- `Workspace` defines `on-create` and `on-acquire` scripts.
- `Job` defines `test-commands` and the optional default `agent` for opencode runs.

## Behavior
- `Load` reads `incrementum.toml` from the repo root and `~/.config/incrementum/config.toml`, then merges them.
- Project values override global values, including explicitly empty strings or lists; missing configs return an empty config.
- TOML decoding errors are surfaced with context.
- `RunScript` executes hook scripts in a target directory.
- Scripts honor a shebang line; otherwise `/bin/bash` is used.
- Script content is passed via stdin, with stdout/stderr forwarded to the caller.
- Job workflows require `job.test-commands` to be present and non-empty.
