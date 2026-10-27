# llmd graph

Show how the store changed over time, or every version of one
document.

## Usage

```
llmd graph          Timeline of the whole store
llmd graph <path>   Version lineage of one document
```

## Rules

- Read-only. No author is needed.
- Without a path, every version event across all documents is
  collected, sorted by time and clustered into bursts of activity.
  Siblings happened close together; depth shows progression through
  time.
- With a path, every version of that document is shown as a flat tree
  with timestamps and authors.
- At a terminal the output is coloured: cyan for paths, yellow for
  versions. Piped, it is plain text, one event per line. `--json`
  returns the events as data.
- An empty store prints `No events found`; a document with no history
  prints `No history found`.

## Examples

```bash
# Show how the store evolved over time
llmd graph

# Show version lineage of a single document
llmd graph plan

# JSON output
llmd graph --json
llmd graph --json plan
```

## Notes

For one document's versions as a table, see `guide history`; to
compare two of them, `guide diff`.
