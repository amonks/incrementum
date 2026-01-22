# Internal Config

## Overview
The config package loads `.incr.toml` files and runs hook scripts.

## Configuration Model
- `Config` holds workspace configuration.
- `Workspace` defines `on-create` and `on-acquire` scripts.

## Behavior
- `Load` reads `.incr.toml` from the repo root and returns an empty config if missing.
- TOML decoding errors are surfaced with context.
- `RunScript` executes hook scripts in a target directory.
- Scripts honor a shebang line; otherwise `/bin/bash` is used.
- Script content is passed via stdin, with stdout/stderr forwarded to the caller.
