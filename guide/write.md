# llmd write

Write content to a document. Creates a new version if the document exists.

## Usage

```bash
llmd write <path> [content]
```

Content can be provided as an argument, from stdin, or from a file.

## Flags

| Flag | Description |
|------|-------------|
| `-f, --file` | Read content from file |

See `llmd guide` for global flags.

## Input Methods

```bash
# Inline content as argument
llmd write docs/readme "# Hello World"

# Read from file with -f flag
llmd write docs/readme -f file.md

# Pipe from echo
echo "# Hello" | llmd write docs/readme

# Pipe from command
cat file.md | llmd write docs/readme

# Redirect from file
llmd write docs/readme < file.md

# Heredoc (use LLMD_DOC delimiter - see note below)
llmd write docs/readme << 'LLMD_DOC'
# Title

Content here.
LLMD_DOC
```

## Examples

```bash
# Simple write
echo "# README" | llmd write docs/readme

# With author attribution (required for LLMs)
echo "content" | llmd write docs/readme -a "claude-code"

# With message
echo "content" | llmd write docs/readme -a "claude-code" -m "Initial draft"

# Multi-line document with code examples
llmd write docs/readme -a "claude-code" << 'LLMD_DOC'
# Installation Guide

Run the following:

```bash
cat << 'EOF'
config content
EOF
```
LLMD_DOC
```

## Heredoc Best Practice

When writing documents that contain code examples with heredocs, use `LLMD_DOC` as your delimiter instead of `EOF`:

```bash
# Good - uses unique delimiter
llmd write docs/guide -a "claude-code" << 'LLMD_DOC'
Content with ```bash << 'EOF' ... EOF``` examples inside
LLMD_DOC

# Bad - will break if content contains EOF
llmd write docs/guide -a "claude-code" << 'EOF'
Content with << 'EOF' examples inside  # Shell parsing error!
EOF
```

**Why?** Standard delimiters like `EOF` commonly appear in code examples. The shell sees the inner `EOF` and terminates the heredoc prematurely, causing parse errors.

**Alternative:** Write to a temp file first, then redirect:

```bash
llmd write docs/readme -a "claude-code" < /path/to/file.md
```

## Notes

- Creates the document if it doesn't exist
- Creates a new version if it does exist (never overwrites)
- All versions are kept and accessible via `llmd history`
- LLMs should always use `-a` flag to identify themselves
- Use `LLMD_DOC` delimiter for heredocs to avoid nested delimiter conflicts
