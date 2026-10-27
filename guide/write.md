# llmd write

Create a document, or replace its content, from standard input.

## Usage

```
echo "text" | llmd write <path>                 Create or update a document
llmd write <path> --message "why" < file.md     Record a version message
```

## Flags

| Flag | Description |
|------|-------------|
| `--message <string>` | Version message describing the change |

## Rules

- An author is required: `--author` for agents and scripts, or the
  configured author at an interactive terminal.
- Content comes from stdin. Empty input is an error; a document always
  has content. Over MCP, pass the body in the `content` field.
- write replaces the whole document. To change part of it use
  `llmd edit` or `llmd sed`.
- An existing document gets a new version; a new path is created at
  version 1. Nothing is overwritten: `llmd history`, `llmd diff` and
  `llmd revert` reach every earlier version.
- Paths look like `notes/plan`: forward slashes, no leading slash. A
  single word is a fine path.

## Examples

```bash
# Pipe content in
echo "Hello, world" | llmd --author claude-code write hello

# Use a heredoc
llmd --author alice write standup <<'TEXT'
- Finished auth module
- Starting API tests
TEXT

# Redirect from a file
llmd --author alice write readme < README.md

# Paths can have hierarchy when grouping related documents
echo "v2 draft" | llmd --author alice write docs/proposal --message "Revised introduction"
```

## Notes

Set the author once for interactive use with `llmd config author
"name"`. Agents and scripts pass `--author` on every call instead,
so the version history says who changed what.
