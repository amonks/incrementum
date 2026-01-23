# Specifications

## Public Packages

| Spec                           | Code                        | Purpose                                                                                                      |
| ------------------------------ | --------------------------- | ------------------------------------------------------------------------------------------------------------ |
| [workspace.md](./workspace.md) | [workspace/](../workspace/) | Jujutsu workspace pool: acquire a workspace to do some isolated work, then release it                        |
| [todo.md](./todo.md)           | [todo/](../todo/)           | Task tracking: command-line JIRA with TODOs stored in a special branch                                       |
| [session.md](./session.md)     | [session/](../session/)     | Session system: a session acquires a workspace to accomplish a single TODO                                   |
| [cli.md](./cli.md)             | [cmd/ii/](../cmd/ii/)       | Describes our architecture: the cli package is a thin wrapper over a go package per-subcommand               |
| [opencode.md](./opencode.md)   |                             | Opencode daemon: send it jobs and monitor their status; all invocations of opencode must pass through        |
| [job.md](./job.md)             |                             | Jobs system: workflow management for using opencode to complete todos (in sessions), with acceptance testing |

## Internal Packages

| Spec                                                   | Code                                                | Purpose                                              |
| ------------------------------------------------------ | --------------------------------------------------- | ---------------------------------------------------- |
| [internal-age.md](./internal-age.md)                   | [internal/age/](../internal/age/)                   | Timing helpers for computed ages and durations       |
| [internal-config.md](./internal-config.md)             | [internal/config/](../internal/config/)             | Load `.incr.toml` configuration and run hook scripts |
| [internal-editor.md](./internal-editor.md)             | [internal/editor/](../internal/editor/)             | `$EDITOR` integration and todo TOML editing flow     |
| [internal-ids.md](./internal-ids.md)                   | [internal/ids/](../internal/ids/)                   | Unique prefix length calculation for IDs             |
| [internal-jj.md](./internal-jj.md)                     | [internal/jj/](../internal/jj/)                     | Go wrapper around jj CLI commands                    |
| [internal-listflags.md](./internal-listflags.md)       | [internal/listflags/](../internal/listflags/)       | Shared Cobra list flags                              |
| [internal-paths.md](./internal-paths.md)               | [internal/paths/](../internal/paths/)               | Default state and workspace paths                    |
| [internal-state.md](./internal-state.md)               | [internal/state/](../internal/state/)               | Shared state file management                         |
| [internal-testsupport.md](./internal-testsupport.md)   | [internal/testsupport/](../internal/testsupport/)   | Integration test helpers for ii/testscript           |
| [internal-ui.md](./internal-ui.md)                     | [internal/ui/](../internal/ui/)                     | CLI formatting helpers for durations and IDs         |
| [internal-validation.md](./internal-validation.md)     | [internal/validation/](../internal/validation/)     | Shared validation formatting helpers                 |
