# llmd ls

List documents in the store.

## Usage

```
llmd ls [flags] [<prefix>]
```

## Flags

| Flag | Description |
|------|-------------|
| `-l` | Long format: shows VER, AUTHOR, DATE, PATH columns |
| `-a` | Include soft-deleted documents |
| `-t` | Sort by time, newest first |
| `-r` | Reverse sort order |

Short flags can be combined: `-lat` is equivalent to `-l -a -t`.

## Examples

```bash
# List all documents
llmd ls

# Long format
llmd ls -l

# List documents under a prefix
llmd ls notes/

# Newest first, long format
llmd ls -lt

# Oldest first, long format
llmd ls -ltr

# Include deleted documents
llmd ls -la
```

## Notes

- Without `-l`, output is one path per line.
- The prefix argument filters to documents whose path starts with the given string.
