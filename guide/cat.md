# llmd cat

Read one or more documents to stdout.

## Usage

```bash
llmd cat <path|key>...
```

Accepts document paths or 8-character keys. Keys are shown in `llmd ls` and `llmd history` output. Multiple files are concatenated in order.

## Flags

| Flag | Description |
|------|-------------|
| `-n, --number` | Number all output lines |
| `-l, --lines` | Line range (e.g., 10:20, 5:, :15) |
| `-v, --version` | Read specific version |
| `-D, --deleted` | Read a deleted document |
| `--raw` | Output raw markdown without rendering |

See `llmd guide` for global flags.

## Examples

```bash
# Read current version by path
llmd cat docs/readme

# Read specific version by key
llmd cat a1b2c3d4

# Show line numbers
llmd cat -n docs/readme

# Read specific line range
llmd cat -l 10:20 docs/readme       # lines 10-20
llmd cat -l 5: docs/readme          # from line 5 to end
llmd cat -l :15 docs/readme         # first 15 lines
llmd cat -n -l 10:20 docs/readme    # with line numbers

# Read specific version
llmd cat docs/readme -v 3

# Read deleted document
llmd cat docs/readme -D

# JSON output with metadata
llmd cat docs/readme -o json

# Redirect to file
llmd cat docs/readme > output.md

# Pipe to another command
llmd cat docs/readme | grep "TODO"

# Output raw markdown (no rendering)
llmd cat --raw docs/readme

# Read multiple files (concatenated)
llmd cat docs/intro docs/setup docs/usage

# Multiple files with JSON output (returns array)
llmd cat docs/a docs/b -o json
```

## JSON Output

Single file returns an object:

```json
{
  "key": "a1b2c3d4",
  "path": "docs/readme",
  "content": "# Hello\n\nContent here.",
  "version": 5,
  "author": "claude-code",
  "message": "Updated intro",
  "created_at": "2024-01-01T00:00:00Z"
}
```

Multiple files return an array:

```json
[
  {"key": "a1b2c3d4", "path": "docs/a", "content": "...", ...},
  {"key": "e5f6g7h8", "path": "docs/b", "content": "...", ...}
]
```

## Notes

- Returns exit code 1 if any document is not found
- Multiple files are output in the order specified
- Use `-D` to read soft-deleted documents
- Use `-v` to access any historical version (applies to all files)
- Output is rendered as formatted markdown when reading a single file in a terminal
- Output is raw markdown when reading multiple files, piping, or redirecting
- Use `--raw` to force raw markdown output in a terminal
