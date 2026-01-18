# llmd sed

Stream editor for documents using sed-style substitution.

## Usage

```bash
llmd sed -i 's/old/new/' <path|key>
llmd sed -i 's/old/new/g' <path|key>   # replace all
```

Accepts either a document path or an 8-character key.

## Examples

```bash
# Basic substitution (first occurrence)
llmd sed -i 's/foo/bar/' docs/readme

# Global substitution (all occurrences)
llmd sed -i 's/foo/bar/g' docs/readme

# Use alternate delimiter for paths/URLs
llmd sed -i 's|http://old|https://new|' docs/config

# With author attribution
llmd sed -i 's/TODO/DONE/' docs/tasks -a "claude"
```

## Syntax

Matches standard sed syntax: `sed -i 's/old/new/[flags]' file`

- `-i` flag is required (in-place editing)
- Expression format: `s<delim>old<delim>new<delim>[flags]`
- Common delimiters: `/`, `|`, `#`, `@`

## Flags

| Flag | Description |
|------|-------------|
| `-i` | Edit in place (required) |

See `llmd guide` for global flags.

## Expression Flags

| Flag | Description |
|------|-------------|
| `g` | Global - replace all occurrences |

## Notes

- Only substitution (`s`) commands are supported
- Without `g` flag, replaces first occurrence only
- Creates a new version of the document
- LLMs should always use `-a` flag to identify themselves
