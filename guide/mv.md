# llmd mv

Move or rename documents.

## Usage

```bash
llmd mv <source> <dest>           # rename single document
llmd mv <source>... <dest>/       # move multiple to prefix
```

## Examples

```bash
# Rename single document
llmd mv docs/readme docs/README

# Move to different location
llmd mv notes/todo docs/todo

# Move multiple documents to prefix
llmd mv docs/a docs/b docs/c archive/

# Move single document into prefix (trailing slash)
llmd mv docs/readme archive/

# JSON output (single returns object, multiple returns array)
llmd mv docs/a docs/b archive/ -o json
```

## Notes

- Preserves all version history
- Fails if destination already exists
- Updates the path for all versions
- Trailing slash on destination signals "move into" prefix mode
- With multiple sources, destination is always treated as a prefix
