# llmd revert

Roll a document back to an earlier version, as a new version.

## Usage

```
llmd revert <path> <version>                     Restore the content of a version
llmd revert <path> <version> --message "why"     Record a message with it
```

## Flags

| Flag | Description |
|------|-------------|
| `--message <string>` | Revert message (default: "Reverted to version N") |

## Rules

- An author is required: `--author` for agents and scripts.
- The version must exist; `llmd history <path>` lists them. The number
  may carry a `v` prefix: `v2` and `2` are the same.
- A deleted document cannot be reverted: `llmd restore` it first.
- Nothing is destroyed. The revert is a new version holding the old
  content, and every version in between stays in the history.

## Examples

```bash
# Revert to version 2
llmd --author claude-code revert draft 2

# Version prefix "v" is accepted
llmd --author claude-code revert draft v2

# Revert with a custom message
llmd --author claude-code revert api 1 --message "Roll back breaking change"
```

## Notes

Because a revert is itself a version, a mistaken revert is undone the
same way: revert to the version before it.
