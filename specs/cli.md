# CLI Architecture

## Purpose

This specification documents the overall CLI architecture and behavior expectations for `incr`. It is intended for implementers adding new subcommands. It does not describe individual command behavior beyond cross-cutting rules.

## Execution Model

- The CLI is structured as a single entry point that dispatches subcommands.
- Command execution is deterministic: given the same inputs and workspace state, the output and side effects must be the same.
- Subcommands are responsible for validating inputs before performing any state changes.
- Errors are reported consistently, using a human-readable message and a non-zero exit code.

## Command Structure

- Commands follow the pattern `incr <noun> <verb> [args] [flags]`.
- Subcommands should avoid side effects unless explicitly requested by the user.
- Output should be concise by default; optional verbose flags may expand detail.

## I/O and Formatting

- Output uses plain text suitable for terminals; avoid ANSI color as a hard requirement.
- Machine-readable output should be provided only when explicitly requested (for example via `--json`).
- Prompts are opt-in; do not block on interactive input unless explicitly requested.

## Error Handling

- All user-facing errors should be actionable and avoid stack traces.
- Unknown commands or flags should include a suggestion when possible.
- Exit codes should be stable and documented alongside command behaviors.

## Testing Expectations

- New subcommands must include tests that cover parsing, validation, and observable behavior.
- Tests should encode expected output and exit codes for common scenarios.
- Do not rely on network access or external processes unless the command explicitly requires them.
