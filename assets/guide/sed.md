# llmd sed

Sed-style substitution on a document.

## Usage

```
llmd sed [-i] 's/old/new/' <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `-i` | Accepted but ignored (always in-place) |

## Examples

```bash
# Basic substitution
llmd sed 's/foo/bar/' notes/draft

# Alternate delimiter (useful when replacing paths)
llmd sed 's|/usr/local|/opt|' config/paths

# With -i (same behaviour, for muscle memory)
llmd sed -i 's/colour/color/' docs/readme
```

## Notes

- Only the `s` command is supported.
- The delimiter is the character immediately after `s` and can be any character.
- Replaces the first occurrence.
- Creates a new version with the change applied.
