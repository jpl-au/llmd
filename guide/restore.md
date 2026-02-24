# llmd restore

Restore a soft-deleted document.

## Usage

```
llmd restore <path>
```

## Examples

```bash
# Delete a document
llmd rm notes/ideas

# Changed your mind — bring it back
llmd restore notes/ideas
```

## Notes

- Only works on documents deleted with `rm` that have not been purged by `vacuum`.
- The restored document retains its full version history.
