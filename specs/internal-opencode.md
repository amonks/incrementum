# Internal Opencode

## Overview
The internal opencode package reads opencode's local storage directory to extract session metadata and transcripts.

## Responsibilities
- Resolve the opencode storage root from `XDG_DATA_HOME/opencode` (falling back to `~/.local/share/opencode`).
- Locate sessions for a repo based on project metadata.
- Load session log entries and prose-only transcripts from stored message parts.
- Select the most relevant session for a run using timestamps and prompt matching.
- When no sessions match, return a not-found error that includes repo path, timing cutoff, total sessions scanned, and the storage directory for debugging.
- Format tool output in session logs with stdout/stderr headings and indented content, preserving long lines.
