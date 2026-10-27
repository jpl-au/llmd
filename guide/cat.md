# llmd cat

Read one or more documents, whole or as a range of lines.

## Usage

```
llmd cat <path> [<path>...]                Print documents, concatenated
llmd cat --offset N --limit M <path>       Print M lines starting at line N
llmd cat --version N <path>                Print an older version
```

## Flags

| Flag | Description |
|------|-------------|
| `--version <int>` | Read a specific version (default: latest) |
| `--offset <int>` | Start reading from this 1-indexed line |
| `--limit <int>` | Maximum number of lines to return |
| `-n` | Number output lines (disables markdown rendering) |

## Rules

- Reads need no author.
- The document must exist and not be deleted; otherwise the call is an
  error. Restore a deleted document first with `llmd restore`.
- `--offset` is 1-indexed; `--offset 1` and no offset are the same. An
  offset past the end returns nothing. `--limit` counts lines after
  the offset is applied.
- Line numbers from `-n` match the source document even when sliced,
  so they line up with `llmd grep --lines -n` output.
- Piped, `--json` or over MCP, the output is the raw markdown source.
  At a terminal it is rendered markdown, unless `-n` is set.
- Several paths are concatenated with newlines, like Unix `cat`.

## Examples

```bash
# Read a whole document
llmd cat readme

# Read a 20-line window starting at line 100, the common case after a grep match
llmd cat --offset 100 --limit 20 spec

# Read just the first 10 lines
llmd cat --limit 10 spec

# Skip the first 50 lines and read the rest
llmd cat --offset 50 spec

# Read with line numbers; with --offset the numbers stay aligned with
# the source document rather than restarting from 1
llmd cat --offset 100 --limit 20 -n spec

# Read a specific version
llmd cat --version 2 notes/meeting

# Read multiple documents (concatenated)
llmd cat monday tuesday wednesday
```

## Notes

cat is the read half of the grep-then-read workflow agents depend on.
After `llmd grep` narrows down which document and roughly where the
match is, `cat` with `--offset` and `--limit` fetches just the
surrounding lines without loading the whole file. Without those flags
cat returns the whole document, matching Unix `cat` for small files.

At a terminal the output is rendered as markdown via glamour. Line
numbering disables rendering because numbered output is no longer
pure markdown.
