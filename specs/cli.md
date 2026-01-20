# CLI Architecture

## Purpose

This specification documents the overall CLI architecture and behavior expectations for `incr`. It is intended for implementers adding new subcommands. It does not describe individual command behavior beyond cross-cutting rules.

## Execution Model

- The CLI is structured as a single entry point that dispatches subcommands.
- Subcommands are responsible for validating inputs before performing any state changes.
- Errors are reported consistently, using a human-readable message and a non-zero exit code.

## Command Structure

- Commands follow the pattern `incr <noun> <verb> [args] [flags]`.
- The CLI layer is a thin wrapper around public packages; command handlers should delegate to package APIs with minimal logic.
- Command interfaces should closely mirror the public APIs they wrap (1:1 when possible).

## I/O and Formatting

- Machine-readable output should be provided only when explicitly requested (for example via `--json`).

## Testing Expectations

- Each package that is wrapped into the CLI should have both regular tests _and_ an e2e test that builds and executes the cli binary
