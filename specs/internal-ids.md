# Internal IDs

## Overview
The ids package computes shortest unique prefixes for identifiers.

## Behavior
- `NormalizeUniqueIDs` lowercases IDs and removes empty or duplicate values.
- `UniquePrefixLengths` normalizes IDs to lowercase and removes duplicates.
- `MatchPrefix` returns the case-preserving ID for a non-empty prefix, and reports missing or ambiguous matches.
- Each ID is assigned the smallest prefix length that is unique among inputs.
- When no shorter unique prefix exists, the full length is returned.
- `Generate` returns a lowercase base32 SHA-256 prefix of the requested length.
- `GenerateWithTimestamp` appends RFC3339Nano timestamps to input before hashing.
- `DefaultLength` is 8, the standard length for generated IDs.
