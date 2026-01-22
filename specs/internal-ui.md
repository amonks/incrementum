# Internal UI

## Overview
The ui package provides formatting helpers for CLI output.

## Time Formatting
- `FormatTimeAgo` returns a compact age string like `2m ago`.
- `FormatTimeAgeShort` returns a compact age string without suffix.
- `FormatDurationShort` formats durations in `s/m/h/d` units.

## ID Highlighting
- `HighlightID` emphasizes unique prefixes when ANSI output is available.
- ANSI output is disabled for `NO_COLOR`, `TERM=dumb`, or non-terminals.
- `UniqueIDPrefixLengths` delegates to `internal/ids` for prefix computation.
