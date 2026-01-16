# llmd mv

Move or rename a document.

## Usage

```bash
llmd mv <old-path> <new-path>
```

## Examples

```bash
# Rename
llmd mv docs/readme docs/README

# Move to different location
llmd mv notes/todo docs/todo

# Restructure
llmd mv api/auth docs/api/auth
```

## Notes

- Preserves all version history
- Fails if destination already exists
- Updates the path for all versions
