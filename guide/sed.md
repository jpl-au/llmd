# llmd sed

Substitute text in one or more documents with a sed-style expression.

## Usage

```
llmd sed 's/old/new/' <path>...       Replace the single occurrence of old
llmd sed 's/old/new/g' <path>...      Replace every occurrence
llmd sed -i 's/old/new/' <path>       Same; -i is accepted for muscle memory
```

## Flags

| Flag | Description |
|------|-------------|
| `-i` | Accepted but ignored; sed always edits in place |
| `--message <string>` | Version message describing the change |

## Rules

- An author is required: `--author` for agents and scripts.
- Only the `s` (substitute) command is supported.
- Without the trailing `g`, the pattern must occur exactly once in the
  document. More than one match is refused with `the old text appears N
  times in <path>`; add `g` to replace them all.
- The delimiter is the character right after `s` and can be any
  character, which helps when the pattern or replacement contains
  slashes.
- With several paths, the substitution is applied to each document in
  turn and stops at the first one that fails. The documents before it
  are already changed.
- Each successful substitution is a new version of that document.

## Examples

```bash
# Basic substitution: old must be unique in the document
llmd --author claude-code sed 's/foo/bar/' draft

# Global substitution: replace every occurrence
llmd --author claude-code sed 's/foo/bar/g' draft

# Alternate delimiter, useful when replacing paths
llmd --author claude-code sed 's|/usr/local|/opt|' config

# With -i (same behaviour)
llmd --author claude-code sed -i 's/colour/color/g' readme
```

## Notes

sed and `llmd edit` do the same job. edit takes literal strings as
two arguments; sed takes one expression and lets you pick the
delimiter. Pick whichever reads better for the change in hand.
