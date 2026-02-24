# llmd grep

Full-text search across document content using FTS5 syntax.

## Usage

```
llmd grep [flags] <query> [<prefix>]
```

## Flags

| Flag | Description |
|------|-------------|
| `-n` | Show line numbers |
| `-l` | Print matching paths only |
| `-c` | Print match count per document |
| `-C N` | Show N context lines (both `-C3` and `-C 3` work) |

## Examples

```bash
# Simple word search
llmd grep database

# Phrase search (exact sequence)
llmd grep '"hello world"'

# Prefix matching
llmd grep 'deploy*'

# Boolean OR
llmd grep 'postgres OR mysql'

# Proximity search (words within 5 tokens of each other)
llmd grep 'NEAR(deploy production)'

# Restrict to a path prefix
llmd grep database notes/

# Show line numbers
llmd grep -n database

# Filenames only
llmd grep -l database

# Count matches per document
llmd grep -c database

# Show 3 lines of context around each match
llmd grep -C3 database
```

## Notes

- Queries use SQLite FTS5 syntax, not regular expressions.
- FTS5 operators: `AND` (implicit), `OR`, `NOT`, `NEAR()`, prefix `*`, phrases `"..."`.
- The optional prefix argument restricts the search to documents under that path.
