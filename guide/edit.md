# llmd edit

Edit a document via search/replace or line range.

## Usage

```bash
llmd edit <path|key> "old" "new"         # search/replace
llmd edit <path|key> -l 5:10 < content   # line replacement
```

Accepts either a document path or an 8-character key.

## Flags

| Flag | Description |
|------|-------------|
| `--old` | Text to find (alternative to positional) |
| `--new` | Text to replace with |
| `-i, --ignore-case` | Case-insensitive matching |
| `-l, --lines` | Line range (e.g., 5:10) |

See `llmd guide` for global flags.

## Search/Replace Mode

Replaces first occurrence only.

```bash
# Positional args (preferred)
llmd edit docs/readme "Hello" "Hi"

# With flags
llmd edit docs/readme --old "Hello" --new "Hi"

# Case-insensitive search/replace
llmd edit docs/readme -i "hello" "Hi"

# With attribution
llmd edit docs/readme "old" "new" -a "claude-code" -m "Fixed typo"
```

## Line Range Mode

Replaces lines start:end (inclusive) with stdin content.

```bash
# Replace lines 5-10 from file
llmd edit docs/readme -l 5:10 < replacement.txt

# Replace with heredoc (use LLMD_DOC delimiter - see note below)
llmd edit docs/readme -l 5:10 << 'LLMD_DOC'
New content for these lines.
LLMD_DOC
```

## Heredoc Best Practice

When using line range mode with heredocs containing code examples, use `LLMD_DOC` as your delimiter:

```bash
# Good - uses unique delimiter
llmd edit docs/guide -l 20:30 -a "claude-code" << 'LLMD_DOC'
## Example

```bash
cat << 'EOF'
config
EOF
```
LLMD_DOC

# Bad - will break if content contains EOF
llmd edit docs/guide -l 20:30 << 'EOF'
Content with << 'EOF' examples  # Shell parsing error!
EOF
```

**Why?** Standard delimiters like `EOF` commonly appear in code examples. The shell sees the inner `EOF` and terminates the heredoc prematurely.

**Alternative:** Write replacement content to a temp file first:

```bash
llmd edit docs/readme -l 5:10 -a "claude-code" < /path/to/replacement.md
```

## Notes

- Search/replace fails if text not found
- Line numbers are 1-indexed
- Creates a new version (original preserved in history)
- LLMs should always use `-a` flag
- Use `LLMD_DOC` delimiter for heredocs to avoid nested delimiter conflicts
