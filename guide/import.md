# llmd import

Bring every `.md` file under a directory into the store as documents.

## Usage

```
llmd import [flags] <dir>
```

## Flags

| Flag | Description |
|------|-------------|
| `--prefix <path>` | Target path prefix for imported documents |
| `--dry-run` | Preview what would be imported without writing |

## Rules

- An author is required: `--author` for agents and scripts, or the
  configured author at an interactive terminal.
- `<dir>` is required. Only `.md` files are imported; other file types
  are skipped.
- File paths relative to the import directory become document paths,
  without the `.md` extension, under `--prefix` when given.
- Unchanged files are skipped. Identical content never creates a new
  version, so there is no way to force one.
- The output lists each document written, then `skipped N unchanged`
  when any were. `--dry-run` prints the same list without writing.

## Examples

```bash
# Import all .md files from a directory
llmd --author alice import ./docs/

# Import under a path prefix
llmd --author alice import --prefix project/docs ./docs/

# Preview without importing
llmd --author alice import --dry-run ./docs/
```

## Notes

`import` is a one-off bulk load from any directory. For a directory
that stays in step with the store, see `guide mirror`; for the other
direction, `guide export`.
