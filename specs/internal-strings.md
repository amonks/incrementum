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
