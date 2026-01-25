# CLI Conventions

The CLI architecture and testing expectations are defined in the Architecture section of `specs/README.md`.
Use this document for additional CLI-specific guidance when it is not already covered there.

## Version Flag

- `ii -version` prints the build identifiers instead of a semantic version.
- Output format:

```text
change_id <jj-change-id>
commit_id <jj-commit-id>
```

- The identifiers are embedded at build time via `-ldflags`.
