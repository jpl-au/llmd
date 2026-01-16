# llmd glob

List document paths matching a glob pattern.

## Usage

```bash
llmd glob [pattern]
```

## Flags

See `llmd guide` for global flags.

## Examples

```bash
# All documents
llmd glob

# Direct children of docs/
llmd glob "docs/*"

# All under docs/ (recursive)
llmd glob "docs/**"

# Specific pattern
llmd glob "*/readme"

# JSON output
llmd glob -o json
```

## Pattern Syntax

| Pattern | Matches |
|---------|---------|
| `*` | Any characters except `/` |
| `**` | Any characters including `/` |
| `?` | Single character |

## Notes

- Queries the database, not filesystem
- Returns document paths for use with `llmd cat`
