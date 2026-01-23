# CLI Architecture

## Purpose

This specification documents the overall CLI architecture and behavior expectations for `ii`. It is intended for implementers adding new subcommands. It does not describe individual command behavior beyond cross-cutting rules.

## Execution Model

- The CLI is structured as a single entry point that dispatches subcommands.
- Subcommands are responsible for validating inputs before performing any state changes.
- Errors are reported consistently, using a human-readable message and a non-zero exit code.

## Command Structure

- Commands follow the pattern `ii <noun> <verb> [args] [flags]`.
- The CLI layer is a thin wrapper around public packages; command handlers should delegate to package APIs with minimal logic.
- Command interfaces should closely mirror the public APIs they wrap (1:1 when possible).
- Flag aliases should be handled via normalization so help shows a single entry; prefer a short `-d` shorthand and accept `--desc` for description when needed.

## I/O and Formatting

- Machine-readable output should be provided only when explicitly requested (for example via `--json`).
- Table output aligns and truncates by visible characters instead of raw byte length.
- Table output normalizes line breaks and tabs to spaces to keep rows single-line.

## Testing Expectations

- Each package that is wrapped into the CLI should have both regular tests _and_ an e2e test that builds and executes the cli binary
- CLI e2e testscript suites live under `cmd/ii/testdata` and are executed from `cmd/ii/*_test.go`
- CLI wrapper unit tests live under `cmd/ii`; go package APIs are tested in their respective module directories
