# llmd revert

Revert a document to a previous version.

## Usage

```
llmd revert <path> <version> [--message "text"]
```

## Flags

| Flag | Description |
|------|-------------|
| `--message` | Commit message (default: "Reverted to version N") |

## Examples

```bash
# Revert to version 2
llmd revert notes/draft 2

# Version prefix "v" is accepted
llmd revert notes/draft v2

# Revert with a custom message
llmd revert docs/api 1 --message "Roll back breaking change"
```

## Notes

- Non-destructive: creates a new version containing the old content.
- Existing versions are preserved in the history.
- Use `llmd history <path>` to see available versions before reverting.
