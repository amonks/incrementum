# Internal Config

## Overview
The config package loads `incrementum.toml` files and runs hook scripts.

## Configuration Model
- `Config` holds workspace and job configuration.
- `Workspace` defines `on-create` and `on-acquire` scripts.
- `Job` defines `test-commands` used by the job workflow.

## Behavior
- `Load` reads `incrementum.toml` from the repo root and returns an empty config if missing.
- TOML decoding errors are surfaced with context.
- `RunScript` executes hook scripts in a target directory.
- Scripts honor a shebang line; otherwise `/bin/bash` is used.
- Script content is passed via stdin, with stdout/stderr forwarded to the caller.
