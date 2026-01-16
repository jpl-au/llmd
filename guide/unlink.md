# llmd unlink

Remove links between documents.

## Usage

```bash
llmd unlink <id>           # remove link by ID
llmd unlink --tag <tag>    # remove all links with tag
```

## Examples

```bash
# List links to find the ID
llmd link --list docs/api
# Output:
# a1b2c3d4  docs/auth
# x9y8z7w6  docs/config [depends-on]

# Remove a specific link by ID
llmd unlink a1b2c3d4

# Remove all links with a tag
llmd unlink --tag depends-on
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--tag` | `-t` | Remove all links with this tag |

## Notes

- Use `llmd link --list <document>` to see link IDs
- Links are soft-deleted (recoverable until vacuum)
- See `llmd guide link` for creating links
