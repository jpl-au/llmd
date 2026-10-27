# llmd mv

Move or rename a document, keeping its history.

## Usage

```
llmd mv <source> <destination>
```

## Rules

- An author is required: `--author` for agents and scripts.
- The source path must exist and the destination path must not.
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
