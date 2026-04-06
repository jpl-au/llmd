# llmd mv

Move or rename a document.

## Usage

```
llmd mv <source> <destination>
```

## Examples

```bash
# Rename a document
llmd mv notes/draft notes/final

# Move to a different directory
llmd mv inbox/task projects/backend/task
```

## Notes

- The full version history moves with the document.
- The source path must exist; the destination path must not.
