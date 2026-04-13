# llmd mv

Move or rename a document.

## Usage

```
llmd mv <source> <destination>
```

## Examples

```bash
# Rename a document
llmd mv draft final

# Move into a hierarchy
llmd mv notes projects/website/notes
```

## Notes

- The full version history moves with the document.
- The source path must exist; the destination path must not.
