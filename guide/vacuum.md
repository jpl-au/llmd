# llmd vacuum

Permanently remove soft-deleted documents, orphaned tags, and orphaned links.

## Usage

```
llmd vacuum
```

## Examples

```bash
# Run vacuum
llmd vacuum
```

## Notes

- Permanently deletes documents that were previously soft-deleted with `llmd rm`.
- Removes tags no longer attached to any document.
- Removes links pointing to or from deleted documents.
- Reclaims disk space in the database.
- This operation is irreversible.
