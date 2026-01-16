# llmd find

Full-text search across documents using SQLite FTS5.

## Usage

```bash
llmd find <query>
```

## Flags

| Flag | Description |
|------|-------------|
| `-p, --path` | Scope search to path prefix |
| `-l, --paths-only` | Only output paths |
| `-D, --deleted` | Search deleted documents only |
| `-A, --all` | Search all (including deleted) |

See `llmd guide` for global flags.

## Examples

```bash
# Basic search (matches word)
llmd find "authentication"

# Prefix matching
llmd find "auth*"

# Multiple words (implicit AND)
llmd find "error handling"

# Boolean operators
llmd find "error OR warning"
llmd find "auth AND NOT jwt"

# Exact phrase
llmd find '"exact phrase"'

# Scope to path
llmd find "TODO" -p docs/

# Paths only (useful for piping)
llmd find "TODO" -l

# Search deleted docs
llmd find "old stuff" -D

# JSON output
llmd find "auth" -o json
```

## FTS5 Query Syntax

| Syntax | Meaning |
|--------|---------|
| `word` | Match documents containing "word" |
| `word*` | Prefix match (auth* matches auth, authentication) |
| `word1 word2` | Both words (implicit AND) |
| `word1 OR word2` | Either word |
| `word1 NOT word2` | First but not second |
| `"exact phrase"` | Exact phrase match |

## Output

Default:
```
docs/api/auth:15: The authentication flow uses JWT tokens...
docs/readme:42: See authentication section for details...
```

Paths only (`-l`):
```
docs/api/auth
docs/readme
```

## Notes

- Uses SQLite FTS5 for fast indexed search
- Searches current versions only (not all history)
- Case-insensitive by default
- For regex pattern matching, use `llmd grep` instead
