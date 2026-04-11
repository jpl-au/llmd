# llmd diff

Compare document versions.

## Usage

```
llmd diff [flags] <path> [<path>]
llmd diff [flags] <path:version> [<path:version>]
```

## Flags

| Flag | Description |
|------|-------------|
| `-C N` | Number of context lines (both `-C3` and `-C 3` work) |
| `--stat` | Show change counts only |
| `--all` | Show full diff without the 500-line truncation cap |

## Examples

```bash
# Compare latest to previous version
llmd diff notes/meeting

# Compare two specific versions of the same document
llmd diff notes/meeting:2 notes/meeting:5

# Compare two different documents
llmd diff notes/monday notes/tuesday

# Show 5 lines of context
llmd diff -C5 notes/meeting

# Counts only
llmd diff --stat notes/meeting
```

## Notes

- With one path: compares the latest version to the previous version.
- With two paths: compares them directly.
- Use `path:version` syntax to pin a specific version (e.g. `docs/readme:3`).
- Output is coloured in a terminal (green for additions, red for removals,
  cyan for hunk headers). Piped output is plain unified diff.
- Diffs over 500 lines are truncated with a summary footer so agents
  don't burn context on huge rewrites. Use `--all` for the full diff,
  or `--stat` when you only need the +/- counts.
