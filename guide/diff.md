# llmd diff

Show differences between document versions or two documents.

## Usage

```bash
llmd diff <path|key> [path2]
llmd diff <path|key> -v <v1:v2>
```

Accepts either a document path or an 8-character key for the first argument.

## Examples

```bash
# Compare latest version with previous version
llmd diff docs/readme

# Compare specific versions
llmd diff docs/readme -v 3:5

# Compare two different documents
llmd diff docs/readme docs/readme-old

# Compare filesystem file with stored document
llmd diff -f ./local.md docs/readme

# Diff a deleted document
llmd diff docs/archived -D

# JSON output (for LLM consumption)
llmd diff docs/readme -o json
```

## Flags

| Long        | Short | Description                          |
|-------------|-------|--------------------------------------|
| `--versions`| `-v`  | Version range (e.g., `3:5`)          |
| `--deleted` | `-D`  | Allow diffing deleted documents      |
| `--file`    | `-f`  | Treat first path as filesystem file  |
| `--raw`     |       | Output without colour                |

See `llmd guide` for global flags.

## Output

```
--- docs/readme v2
+++ docs/readme v3
- Old line that was removed
+ New line that was added
  Unchanged context line
```

**JSON output:**

```json
{
  "old": "docs/readme v2",
  "new": "docs/readme v3",
  "diff": "--- docs/readme v2\n+++ docs/readme v3\n- Old line\n+ New line\n"
}
```

## Behaviour

- Without arguments: compares latest version with previous version
- With `-v 3:5`: compares version 3 with version 5
- With two paths: compares the latest versions of two different documents
- With `-f`: reads first argument from filesystem, compares with second argument from store
- Deleted documents require `--deleted` flag
