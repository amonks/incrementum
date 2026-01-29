# Testing

## Overview
Incrementum tests are organized into tiers that exercise real behavior instead of mocks. Tests should model the way the CLI and jobs interact with jj repositories, filesystem state, and opencode.

## Test Tiers

### Unit Tests
- Pure Go logic stays in package-local tests next to the code.
- Focus on core domain logic, formatting helpers, and validation rules.
- Snapshot tests for text formatting live under `job/testdata/snapshots` and compare rendered prompts, commit messages, and log output against curated fixtures. Update these files manually when formatting rules change.

### Integration Tests
- Integration tests use real binaries and on-disk state (jj, opencode storage, workspaces).
- Use helpers in `internal/testsupport` to set up temp home/state directories.

### Realistic End-to-End Tests
- End-to-end tests create real jj repositories, seed commits, and set a plausible `main` bookmark.
- They run the `ii` CLI, create todos, and complete them via job flows.
- Scripts verify todo state, filesystem changes, and jj history rather than mocked results.

## Testscript and txtar Suites
- CLI e2e suites live under `cmd/ii/testdata` and are executed from `cmd/ii/*_test.go`.
- Each test is a txtar archive with a phase-oriented script plus supporting files.
- Testscript `exec` runs the real `ii` binary built via `BuildII`.

## Supporting Utilities
- `internal/testsupport.BuildII` builds the CLI once per test run.
- `internal/testsupport.SetupScriptEnv` prepares `HOME`, state, and workspace roots for testscript.
- `internal/testsupport.CmdEnvSet` and `internal/testsupport.CmdTodoID` help plumb script output into later steps.
- Opencode stub binaries live inside txtar archives to keep job runs deterministic.
