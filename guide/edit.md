# llmd edit

Search and replace within a document.

## Usage

```
llmd edit <path> <old> <new> [--message "text"]
```

## Flags

| Flag | Description |
|------|-------------|
| `--message` | Commit message describing the change |

## Examples

```bash
# Replace text in a document
llmd edit notes/todo "buy milk" "buy oat milk"

# Replace with a commit message
llmd edit docs/readme "WIP" "Released" --message "Mark as released"
```

## Notes

- Replaces the first occurrence of `old` with `new`.
- Creates a new version with the change applied.
- Use `sed` for sed-style expressions instead of positional arguments.
