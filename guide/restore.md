# llmd restore

Bring back a document that was soft-deleted with `llmd rm`.

## Usage

```
llmd restore <path>
```

## Rules

- An author is required: `--author` for agents and scripts.
- The document must have been deleted with `llmd rm` and not yet
  purged by `llmd vacuum`. After a vacuum there is nothing to restore.
- The restored document keeps its full version history.

## Examples

```bash
# Delete a document
llmd --author alice rm ideas

# Changed your mind: bring it back
llmd --author alice restore ideas
```

## Notes

Deleted documents still show in `llmd ls -a`, which is the way to find
a path you no longer remember. See `guide rm` and `guide vacuum`.
