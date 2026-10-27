# llmd link

Create a directed link from one document to another, or list a
document's links.

## Usage

```
llmd link <from> <to>                 Link from one document to another
llmd link --label "text" <from> <to>  Link with a label
llmd link <path>                      List outgoing links from a document
llmd link --in <path>                 List incoming links
llmd link --both <path>               List links in both directions
```

## Flags

| Flag | Description |
|------|-------------|
| `--label <string>` | Set a label on the link |
| `--in` | Show incoming links instead of outgoing |
| `--both` | Show links in both directions |

## Rules

- Creating a link needs an author; listing does not.
- Both documents must exist. A document cannot link to itself, and a
  link that already exists is refused.
- Links are directional, from source to target. Remove one with
  `llmd unlink <from> <to>`.
- Listing prints one link per line as `from -> to`, with the label in
  brackets when there is one.

## Examples

```bash
# Create a link from one document to another
llmd --author alice link meeting action-items

# Create a labelled link
llmd --author alice link --label "follow-up" meeting action-items

# List outgoing links from a document
llmd link meeting

# List incoming links to a document
llmd link --in action-items

# List links in both directions
llmd link --both meeting
```

## Notes

Tasks can also be linked to documents with `llmd task link <id>
<path>`. Linked documents appear in `llmd review` output, giving
reviewers context. `llmd vacuum` removes orphaned links pointing to
deleted documents.
