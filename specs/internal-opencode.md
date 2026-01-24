# Internal Opencode

## Overview
The internal opencode package reads opencode's local storage directory to extract session metadata and transcripts.

## Responsibilities
- Resolve the default opencode storage root.
- Locate sessions for a repo based on project metadata.
- Load session log entries and prose-only transcripts from stored message parts.
- Select the most relevant session for a run using timestamps and prompt matching.
