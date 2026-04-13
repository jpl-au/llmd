# llmd rm

Soft-delete a document.

## Usage

```
llmd rm <path>
```

## Examples

```bash
# Delete a document
llmd rm old-draft

# Restore it later if needed
llmd restore old-draft

# Permanently purge all soft-deleted documents
llmd vacuum
```

## Notes

- The document is soft-deleted and hidden from `ls` output.
- Content and version history are preserved until `vacuum` is run.
- Use `restore` to recover a soft-deleted document.
