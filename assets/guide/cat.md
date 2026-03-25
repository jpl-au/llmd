# llmd cat

Read one or more documents and print their content.

## Usage

```
llmd cat <path> [<path>...]
```

## Flags

| Flag | Description |
|------|-------------|
| `-n` | Show line numbers |
| `--version N` | Read a specific version (default: latest) |

## Examples

```bash
# Read a document
llmd cat notes/meeting

# Read with line numbers
llmd cat -n notes/meeting

# Read a specific version
llmd cat --version 2 notes/meeting

# Read multiple documents (concatenated)
llmd cat notes/monday notes/tuesday notes/wednesday
```

## Notes

- Multiple paths are concatenated with newlines, matching Unix `cat` behaviour.
- Line numbers are right-aligned to the width of the highest line number.
- Version 0 (the default) means the latest version.
