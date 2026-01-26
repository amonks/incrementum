# Internal Strings

## Overview

The internal strings package provides shared string normalization helpers used
across subsystems.

## NormalizeWhitespace

- Collapses all runs of whitespace into single ASCII spaces.
- Uses Unicode whitespace definition via `strings.Fields`.
- Returns an empty string when the input contains only whitespace.

## NormalizeLower

- Lowercases the input string using `strings.ToLower`.

## NormalizeLowerTrimSpace

- Trims surrounding whitespace with `strings.TrimSpace` and then lowercases.
- Does not alter inner whitespace beyond trimming the edges.

## NormalizeNewlines

- Replaces CRLF and CR sequences with LF.
- Used to normalize multi-line inputs before trimming or formatting.

## TrimTrailingCarriageReturn

- Removes a single trailing `\r` from a line when present.
- Used to normalize CRLF line endings when working with line-split content.

## TrimTrailingNewlines

- Removes trailing `\r` and `\n` characters from the end of a string.
- Used when normalizing multi-line input/output before formatting.

## TrimTrailingWhitespace

- Removes trailing Unicode whitespace characters from a string.
- Used to normalize line endings when trimming formatted commit message lines.

## TrimTrailingSlash

- Removes trailing `/` characters from the end of a string.
- Used to normalize base URLs before building API or web client paths.
