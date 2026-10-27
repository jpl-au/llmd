# llmd unlink

Remove a link between two documents.

## Usage

```
llmd unlink <from> <to>
```

## Rules

- An author is required: `--author` for agents and scripts.
- Both the source and target paths must be given, in the direction the
  link was made. The link must exist.

## Examples

```bash
# Remove a link
llmd --author alice unlink notes/meeting notes/action-items
```

## Notes

See `guide link` for creating and listing links.
