# llmd diff

Compare a document with its previous version, or any two documents or
versions.

## Usage

```
llmd diff <path>                          Latest version against the previous
llmd diff <path:version> <path:version>   Two versions, of one document or two
llmd diff <path> <path>                   Two documents, latest versions
```

A document key works anywhere a path does, e.g. `abc123def:2`.

## Flags

| Flag | Description |
|------|-------------|
| `-C <int>` | Lines of context (`-C3` and `-C 3` both work) |
| `--stat` | Show change counts only |
| `--all` | Show the full diff, without the 500-line cap |

## Rules

- Reads need no author.
- With one path, the document needs at least two versions; a document
  with one version has nothing to compare.
- Diffs over 500 lines are cut off with a footer giving the number of
  lines left out and the total +/- counts. `--all` shows everything; `--stat` shows
  only the counts.
- Piped, the output is a plain unified diff. At a terminal additions
  are green, removals red, hunk headers cyan.

## Examples

```bash
# Compare latest to previous version
llmd diff plan

# Compare two specific versions of the same document
llmd diff plan:2 plan:5

# Compare two different documents
llmd diff monday tuesday

# Same as above, using a document key
llmd diff abc123def:2 abc123def:5

# Show 5 lines of context
llmd diff -C5 plan

# Counts only
llmd diff --stat plan
```

## Notes

The cap exists because a large rewrite can produce thousands of diff
lines, which is poor value for an agent's context. The footer says how
much was left out so the caller can decide whether `--all` is worth
it. Use `llmd history <path>` to find version numbers.
