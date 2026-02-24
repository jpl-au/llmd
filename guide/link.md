# llmd link

Create and list links between documents.

## Usage

```
llmd link <from> <to>
llmd link <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `--label` | Set a label on the link |
| `--in` | Show incoming links instead of outgoing |
| `--both` | Show links in both directions |

## Examples

```bash
# Create a link from one document to another
llmd link notes/meeting notes/action-items

# Create a labelled link
llmd link --label "follow-up" notes/meeting notes/action-items

# List outgoing links from a document
llmd link notes/meeting

# List incoming links to a document
llmd link --in notes/action-items

# List links in both directions
llmd link --both notes/meeting
```

## Notes

- Links are directional: from source to target.
- Use `llmd unlink` to remove a link.
- `llmd vacuum` removes orphaned links pointing to deleted documents.
