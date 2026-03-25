# llmd history

Show the version history of a document.

## Usage

```
llmd history [flags] <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `-n N` | Limit to the most recent N versions (both `-n5` and `-n 5` work) |

## Examples

```bash
# Show full history
llmd history notes/meeting

# Show the last 3 versions
llmd history -n3 notes/meeting

# JSON output
llmd history --json notes/meeting
```

## Notes

- Output is a table with columns: Version, Author, Date, Message.
- Versions are numbered from 1 (oldest) upwards.
