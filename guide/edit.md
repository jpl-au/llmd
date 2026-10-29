# llmd edit

Replace one string with another in a document, as a new version.

## Usage

```
llmd edit <path> <old> <new>            Replace the single occurrence of old
llmd edit --all <path> <old> <new>      Replace every occurrence
```

## Flags

| Flag | Description |
|------|-------------|
| `--message <string>` | Version message describing the change |
| `--all` | Replace every occurrence instead of requiring a unique match |

## Rules

- An author is required: `--author` for agents and scripts.
- `old` must appear exactly once. If it appears more than once, the
  edit is refused with `the old text appears N times in <path>`: add
  surrounding context to `old` until it is unique, or pass `--all`.
- If `old` does not appear at all, the edit is refused with `the old
  text is not in <path>`. Re-read the document; the text has moved on.
- If `old` and `new` are the same, the edit is refused with `the old and
  new text are the same`.
- Matching is literal. For a pattern with a flexible delimiter use
  `llmd sed`.
- Each successful edit is a new version; nothing is overwritten.

## Examples

```bash
# Unique match
llmd --author claude-code edit todo "buy milk" "buy oat milk"

# Ambiguous match: widen old with its context until it is unique
llmd --author claude-code edit config "port: 8080" "port: 9090"

# Substitute every occurrence
llmd --author claude-code edit --all readme "WIP" "Released"

# With a version message
llmd --author claude-code edit readme "WIP" "Released" --message "Mark as released"
```

## Errors

| Error | Meaning |
|-------|---------|
| `the old text is not in <path>` | `old` does not appear in the document. Re-read the file; your copy is stale. |
| `the old text appears N times in <path>` | `old` matches more than one place. Add context, or use `--all`. |
| `the old and new text are the same` | The edit would change nothing. |

## Notes

edit follows the convention every agent tool uses: the search string
must appear exactly once, so the call either succeeds in one
predictable place or fails loudly.
