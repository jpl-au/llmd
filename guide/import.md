# llmd import

Bulk import markdown files from filesystem.

## Usage

```bash
llmd import <filesystem-path>
```

## Flags

| Flag | Description |
|------|-------------|
| `-t, --to` | Target path prefix |
| `-F, --flat` | Flatten directory structure |
| `-n, --dry-run` | Show what would be imported |
| `-H, --include-hidden` | Include hidden files/dirs |
| `-a, --author` | Version attribution |
| `-m, --message` | Version message |

## Examples

```bash
# Import directory
llmd import ./docs/

# Import under prefix
llmd import ./docs/ -t project/notes

# Flatten structure
llmd import ./docs/ -F

# Dry run first
llmd import ./docs/ -n

# With attribution
llmd import ./docs/ -m "Initial import"
```

## Mapping

```
Filesystem:           Database:
./docs/
  readme.md      ->   docs/readme
  api/
    auth.md      ->   docs/api/auth
    users.md     ->   docs/api/users
```

## Notes

- Only imports `.md` files
- Strips `.md` extension from paths
- Skips hidden files/directories by default
- Use `-n` to preview before importing
- LLMs should always use `-a` flag to identify themselves
