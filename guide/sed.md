# llmd sed

Sed-style substitution on a document.

Like `edit`, the search pattern must occur exactly once in the
document by default - ambiguous matches fail rather than silently
editing the wrong place. Append the trailing `g` flag
(`s/old/new/g`) to substitute every occurrence.

## Usage

```
llmd sed [-i] 's/old/new/[g]' <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `-i` | Accepted but ignored (always in-place) |

## Examples

```bash
# Basic substitution - old must be unique in the doc
llmd sed 's/foo/bar/' notes/draft

# Global substitution - replace every occurrence
llmd sed 's/foo/bar/g' notes/draft

# Alternate delimiter (useful when replacing paths)
llmd sed 's|/usr/local|/opt|' config/paths

# With -i (same behaviour, for muscle memory)
llmd sed -i 's/colour/color/g' docs/readme
```

## Notes

- Only the `s` (substitute) command is supported.
- The delimiter is the character immediately after `s` and can be
  any character - useful when the pattern or replacement contains
  forward slashes.
- Without the `g` flag the match must be unique; with it, every
  occurrence is replaced. Ambiguous matches without `g` fail with
  `search string is not unique`.
- Creates a new version with the change applied.
