# llmd find

Full-text search returning matching paths only.

## Usage

```
llmd find <query> [<prefix>]
```

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

# Restrict to a path prefix
llmd find database notes/
```

## Notes

- Uses the same FTS5 syntax as `llmd grep`. See `llmd guide grep` for query details.
- Output is one path per line, suitable for piping.
