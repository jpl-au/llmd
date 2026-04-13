# llmd edit

Search and replace within a document.

`edit` matches the convention every AI tool uses: the search string
must appear **exactly once** in the document so the call either
succeeds deterministically or fails loudly. If the match is
ambiguous, expand the search string with surrounding context until
it's unique, or pass `--all` to substitute every occurrence.

## Usage

```
llmd edit [flags] <path> <old> <new>
```

## Flags

| Flag | Description |
|------|-------------|
| `--message` | Version message describing the change |
| `--all` | Replace every occurrence instead of requiring a unique match |

## Examples

```bash
# Unique match - just works
llmd edit todo "buy milk" "buy oat milk"

# Ambiguous match - expand with context to disambiguate
llmd edit config "port: 8080" "port: 9090"

# Substitute every occurrence
llmd edit --all readme "WIP" "Released"

# With a version message
llmd edit readme "WIP" "Released" --message "Mark as released"
```

## Errors

| Error | Meaning |
|-------|---------|
| `no match found` | `old` does not appear in the document. Re-read the file - your mental model is stale. |
| `search string is not unique` | `old` matches multiple places. Add context to disambiguate, or use `--all`. |
| `old and new are identical` | The edit would produce no change. |

## Notes

- Creates a new version with the change applied.
- Use `sed` for delimiter-flexible expressions (`s/old/new/`).
- The `Data` field of the response (via `--json` or MCP) carries the
  structured document metadata for agents that need it.
