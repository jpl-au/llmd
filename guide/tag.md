# llmd tag

Manage document tags.

## Usage

```
llmd tag
llmd tag <path>
llmd tag <path> <name>
llmd tag -d <path> <name>
llmd tag -f <name>
```

## Flags

| Flag | Description |
|------|-------------|
| `-d` | Delete a tag from a document |
| `-f` | Find all documents with a given tag |

## Examples

```bash
# List all tags with counts
llmd tag

# List tags on a document
llmd tag notes/meeting

# Add a tag to a document
llmd tag notes/meeting important

# Remove a tag from a document
llmd tag -d notes/meeting important

# Find all documents with a tag
llmd tag -f important
```

## Notes

- Tags are simple strings with no hierarchy.
- `llmd vacuum` removes orphaned tags that are no longer attached to any document.
