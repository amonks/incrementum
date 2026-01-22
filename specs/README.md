# Specifications

| Spec                           | Code                        | Purpose                                                                                                      |
| ------------------------------ | --------------------------- | ------------------------------------------------------------------------------------------------------------ |
| [workspace.md](./workspace.md) | [workspace/](../workspace/) | Jujutsu workspace pool: acquire a workspace to do some isolated work, then release it                        |
| [todo.md](./todo.md)           | [todo/](../todo/)           | Task tracking: command-line JIRA with TODOs stored in a special branch                                       |
| [session.md](./session.md)     | [session/](../session/)     | Session system: a session acquires a workspace to accomplish a single TODO                                   |
| [cli.md](./cli.md)             | [cmd/ii/](../cmd/ii/)       | Describes our architecture: the cli package is a thin wrapper over a go package per-subcommand               |
| [opencode.md](./opencode.md)   |                             | Opencode daemon: send it jobs and monitor their status; all invocations of opencode must pass through        |
| [job.md](./job.md)             |                             | Jobs system: workflow management for using opencode to complete todos (in sessions), with acceptance testing |
