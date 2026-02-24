# llmd mirror

One-way push of documents to the filesystem as `.md` files under `.llmd/mirror/`.

## Usage

```
llmd mirror [<prefix>]
```

## Examples

```bash
# Mirror all documents
llmd mirror

# Mirror only documents under a prefix
llmd mirror notes/
```

## Notes

- Files are written to `.llmd/mirror/`, preserving document path structure.
- Unchanged files are skipped.
- Stale files (documents since deleted or renamed) are removed from the mirror directory.
- This is a one-way operation; changes to mirrored files are not imported back.
