# llmd history

Show the versions of a document, newest first.

## Usage

```
llmd history <path>            The 10 most recent versions
llmd history -n N <path>       The N most recent versions
llmd history --all <path>      Every version
```

## Flags

| Flag | Description |
|------|-------------|
| `-n <int>` | Maximum versions to show (default 10) |
| `--all` | Show every version, no limit |

## Rules

- Reads need no author.
- Only the 10 newest versions are shown unless `-n` or `--all`, so a
  heavily edited document does not fill an agent's context.
- Versions are numbered from 1, the oldest, upwards. A document with
  no versions prints `No history found`.
- Piped, the output is tab-separated lines with a header line: VER,
  AUTHOR, DATE, MESSAGE. At a terminal it is a bordered table.

## Examples

```bash
# Recent 10 versions (default)
llmd history plan

# Last 3 versions
llmd history -n3 plan

# Every version
llmd history --all plan

# JSON output
llmd history --json notes/meeting
```

## Notes

Use the version numbers with `llmd cat --version N`, `llmd diff
path:N path:M` and `llmd revert path N`.
