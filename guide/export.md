# llmd export

Export documents from store to filesystem.

## Usage

```bash
llmd export <doc-path|key> <filesystem-path>
```

Accepts either a document path or an 8-character key. When given a key, exports that specific version.

## Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing files |
| `-v, --version` | Export specific version |

## Examples

```bash
# Export directory
llmd export docs/ ./output/

# Export everything
llmd export / ./backup/

# Export single file
llmd export docs/readme ./readme.md

# Overwrite existing
llmd export docs/ ./output/ --force

# Export specific version by number
llmd export docs/readme ./old.md -v 3

# Export specific version by key
llmd export a1b2c3d4 ./old.md
```

## Mapping

```
Database:             Filesystem:
docs/readme      ->   ./output/docs/readme.md
docs/api/auth    ->   ./output/docs/api/auth.md
```

## Notes

- Adds `.md` extension to exported files
- Creates directories as needed
- Fails if file exists (use `--force` to overwrite)
- Single doc: destination can be a file path
- Multiple docs: destination must be a directory
