# llmd history

Show version history for a document.

## Usage

```bash
llmd history <path|key>
```

Accepts either a document path or an 8-character key. When given a key, shows history for the document that key belongs to.

## Flags

| Flag | Description |
|------|-------------|
| `-n, --limit` | Number of versions to show |
| `-d, --diff` | Show diffs between versions |
| `-D, --deleted` | Show history for deleted doc |

See `llmd guide` for global flags.

## Examples

```bash
# Full history
llmd history docs/readme

# Last 5 versions
llmd history docs/readme -n 5

# History of deleted doc
llmd history docs/old -D

# Show diffs between versions
llmd history docs/readme -d

# JSON output
llmd history docs/readme -o json
```

## Output

```
KEY       VER   DATE              AUTHOR       MESSAGE
a1b2c3d4  v5    2024-01-15 10:30  claude-code  "Refactored auth section"
e5f6g7h8  v4    2024-01-14 16:00  james        "Fixed typo"
i9j0k1l2  v3    2024-01-14 09:00  james        -
m3n4o5p6  v2    2024-01-10 11:00  claude-code  "Added examples"
q7r8s9t0  v1    2024-01-10 08:00  james        "Initial draft"
```

## Notes

- Use `llmd cat <key>` or `llmd cat <path> -v N` to read a specific version
- Keys uniquely identify each version and can be used with most commands
- Versions are never deleted unless vacuumed
