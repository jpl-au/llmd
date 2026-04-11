# llmd cat

Read one or more documents and print their content.

llmd is an AI-first tool, and cat is the read half of the
grep-then-read workflow agents depend on. After `grep` narrows down
which document and roughly where the match is, use `cat` with
`--offset` and `--limit` to fetch just the surrounding lines without
loading the whole file. Without those flags cat returns the whole
document, matching Unix `cat` for small files.

## Usage

```
llmd cat [flags] <path> [<path>...]
```

## Flags

| Flag | Description |
|------|-------------|
| `--offset N` | Start reading from this 1-indexed line |
| `--limit N` | Maximum number of lines to return |
| `--version N` | Read a specific version (default: latest) |
| `-n` | Show line numbers |

## Examples

```bash
# Read a whole document
llmd cat notes/meeting

# Read a 20-line window starting at line 100 (the AI-first
# common case after a grep match)
llmd cat --offset 100 --limit 20 api/spec

# Read just the first 10 lines
llmd cat --limit 10 api/spec

# Skip the first 50 lines and read the rest
llmd cat --offset 50 api/spec

# Read with line numbers; when combined with --offset the numbers
# stay aligned with the source document, not restarting from 1
llmd cat --offset 100 --limit 20 -n api/spec

# Read a specific version
llmd cat --version 2 notes/meeting

# Read multiple documents (concatenated)
llmd cat notes/monday notes/tuesday notes/wednesday
```

## Output

On an interactive terminal the output is rendered as markdown via
glamour. Pipes, redirects, `--json`, and MCP get raw markdown source.
Line numbering (`-n`) disables markdown rendering because numbered
output is no longer pure markdown.

## Notes

- Multiple paths are concatenated with newlines, matching Unix `cat`.
- `--offset` is 1-indexed; `--offset 1` and no offset are equivalent.
- An offset past the end of the document returns an empty result.
- `--limit` caps the number of lines *after* the offset is applied.
- Line numbers with `-n` match the source document even when sliced,
  so an agent can correlate them with `grep --lines -n` output.
