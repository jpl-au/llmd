# llmd find

Full-text search that prints only the paths of matching documents.

## Usage

```
llmd find <query> [<path>]      Paths of documents matching the query, under an optional prefix
```

## Flags

| Flag | Description |
|------|-------------|
| `-n`, `--limit <int>` | Maximum paths to return (default 500) |
| `--all` | Return every match, no limit |

## Rules

- Reads need no author.
- The query uses FTS5 syntax, the same as `llmd grep`: bare words are
  an implicit AND, `OR` and `NOT` are operators, `"a phrase"` matches
  exactly, `word*` matches a prefix.
- At most 500 paths are returned unless `--limit N` or `--all`, so a
  broad query on a large store stays bounded. No match prints nothing.
- Output is one path per line, ready for piping.

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

find is `llmd grep -l` under another name, for when only the paths
matter. See `guide grep` for the query syntax in full and for
searching with content.
