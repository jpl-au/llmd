# llmd find

Full-text search returning matching paths only.

## Usage

```
llmd find [flags] <query> [<path>]
```

## Flags

| Flag | Description |
|------|-------------|
| `--limit N` | Maximum paths to return (default 500) |
| `--all` | Return every match, no limit |

## Examples

```bash
# Find documents containing a word
llmd find database

# Phrase search
llmd find '"error handling"'

# Prefix matching
llmd find 'deploy*'

# Boolean OR
llmd find 'postgres OR mysql'

# Restrict to documents under a path
llmd find database notes/
```

## Notes

- Uses the same FTS5 syntax as `llmd grep`. See `llmd guide grep` for query details.
- Output is one path per line, suitable for piping.
- Defaults to 500 matches so queries on large stores stay bounded.
  Use `--all` or `--limit` to change the cap.
