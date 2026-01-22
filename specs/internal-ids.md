# Internal IDs

## Overview
The ids package computes shortest unique prefixes for identifiers.

## Behavior
- `UniquePrefixLengths` normalizes IDs to lowercase and removes duplicates.
- Each ID is assigned the smallest prefix length that is unique among inputs.
- When no shorter unique prefix exists, the full length is returned.
