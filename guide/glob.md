# llmd glob

List documents whose paths match a shell-style pattern.

## Usage

```
llmd glob <pattern>
```

## Flags

| Flag | Description |
|------|-------------|
| `--limit <int>` | Maximum paths to return (default 500) |
| `--all` | Return every match, no limit |

## Rules

- Reads need no author.
- `*` matches any characters within one path segment, `**` matches
  across segments (zero or more levels), `?` matches one character.
  Quote the pattern so the shell does not expand it first.
- At most 500 paths are returned unless `--limit N` or `--all`, so a
  pattern like `**/*` on a large store stays bounded. No match prints
  nothing.
- Output is one path per line.

## Examples

```bash
# Match all documents one level under notes/
llmd glob 'notes/*'

# Match all documents recursively under notes/
llmd glob 'notes/**'

# Match a single character
llmd glob 'notes/meeting-?'

# Match all documents ending in a suffix
llmd glob '**/*-draft'
```

## Notes

glob matches paths only. To search by content use `llmd grep` or
`llmd find`; to list everything under a prefix, `llmd ls <prefix>` is
enough.
