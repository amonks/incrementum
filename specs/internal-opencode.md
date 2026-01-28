# Internal Opencode

## Overview
The internal opencode package reads opencode's local storage directory to extract session metadata and transcripts.

## Responsibilities
- Resolve the opencode storage root from `XDG_DATA_HOME/opencode` (falling back to `~/.local/share/opencode`).
- Locate sessions for a repo based on project metadata, resolving symlinks so
  paths match when the repo is accessed via a symlink.
- Aggregate sessions across every project record that matches the repo worktree
  so stale project ids do not hide new session metadata.
- Load session log entries and prose-only transcripts from stored message parts.
- Select the most relevant session for a run using timestamps (created or updated) scoped to the current repo, then prompt matching and latest-session fallback within the repo when no sessions meet the cutoff window.
- Missing storage directories are treated as empty rather than fatal errors.
- When no sessions match, return a not-found error that includes repo path, timing cutoff, total sessions scanned, and the storage directory for debugging.
- Format tool output in session logs with stdout/stderr headings and indented content, preserving long lines.
