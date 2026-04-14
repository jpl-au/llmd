# llmd graph

Visualise the evolution of the document store.

Without arguments it collects every version event across all documents,
sorts them chronologically, and clusters them into bursts of activity.
With a path argument it shows the version lineage of a single document.

## Usage

```
llmd graph [path]
```

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

- The timeline view clusters events into bursts of activity. Siblings
  happened close together; depth shows progression through time.
- The lineage view shows every version of a single document as a flat
  tree with timestamps and authors.
- Terminal output is coloured (cyan for paths, yellow for versions).
  Piped output is plain text.
