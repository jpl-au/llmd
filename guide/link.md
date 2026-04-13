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
llmd link meeting action-items

# Create a labelled link
llmd link --label "follow-up" meeting action-items

# List outgoing links from a document
llmd link meeting

# List incoming links to a document
llmd link --in action-items

# List links in both directions
llmd link --both meeting
```

## Notes

- Links are directional: from source to target.
- Use `llmd unlink` to remove a link.
- Tasks can also be linked to documents with `llmd task link <id> <path>`.
  Linked documents appear in `review` output, giving reviewers context.
- `llmd vacuum` removes orphaned links pointing to deleted documents.
