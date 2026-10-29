# llmd mv

Move or rename a document, keeping its history.

## Usage

```
llmd mv <source> <destination>
```

## Rules

- An author is required: `--author` for agents and scripts.
- The source must be a live document. The destination must hold no
  document, live or deleted: restore or vacuum a deleted one first.
- The full version history moves with the document.

## Examples

```bash
# Rename a document
llmd --author alice mv draft final

# Move into a hierarchy
llmd --author alice mv notes projects/website/notes
```

## Notes

Paths are plain strings, so moving into a "folder" is just giving the
document a longer path. Nothing needs to exist above it first.
