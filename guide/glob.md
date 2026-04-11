# llmd glob

Match document paths using shell-style glob patterns.

## Usage

```
llmd glob [flags] <pattern>
```

## Flags

| Flag | Description |
|------|-------------|
| `--limit N` | Maximum paths to return (default 500) |
| `--all` | Return every match, no limit |

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

- `*` matches any characters within a single path segment.
- `**` matches across path segments (zero or more levels).
- `?` matches exactly one character.
- Output is one path per line.
- Defaults to 500 matches so patterns like `**/*` on large stores
  stay bounded. Use `--all` or `--limit` to change the cap.
