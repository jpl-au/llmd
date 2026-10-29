# llmd tag

Add, remove, list or search by tags on documents.

## Usage

```
llmd tag                       List all tags with counts
llmd tag <path>                List the tags on a document
llmd tag <path> <name>         Add a tag to a document
llmd tag -d <path> <name>      Remove a tag from a document
llmd tag -f <name>             List documents carrying a tag
```

## Flags

| Flag | Description |
|------|-------------|
| `-d, --delete` | Remove a tag from a document |
| `-f, --find` | Find all documents with a tag |

## Rules

- Adding or removing a tag needs an author; listing and finding do not.
- The document must exist. Adding a tag it already has, or removing
  one it does not have, is an error.
- Tag names are lowercase letters, digits and hyphens, up to 64
  characters, with no hierarchy: `needs-review`, not `a/b`.
- Listing prints one item per line: `name (count)` for all tags, the
  tag names for a document, the paths for `-f`.

## Examples

```bash
# List all tags with counts
llmd tag

# List tags on a document
llmd tag plan

# Add a tag to a document
llmd --author alice tag plan important

# Remove a tag from a document
llmd --author alice tag -d plan important

# Find all documents with a tag
llmd tag -f important
```

## Notes

Tags work on any document, including task spec documents. Use them to
track state beyond the board column, such as `needs-design` or
`has-migration`. `llmd vacuum` removes orphaned tags that are no
longer attached to any document.
