# llmd write

Write a document from standard input.

## Usage

```
llmd write <path> [--message "text"]
```

## Flags

| Flag | Description |
|------|-------------|
| `--message` | Commit message describing the change |

## Examples

```bash
# Pipe content in
echo "Hello, world" | llmd write greetings/hello

# Use a heredoc
llmd write notes/standup <<'EOF'
- Finished auth module
- Starting API tests
EOF

# Redirect from a file
llmd write docs/readme < README.md

# Write with a commit message
echo "v2 draft" | llmd write docs/proposal --message "Revised introduction"
```

## Notes

- If the document already exists, a new version is created.
- If the document does not exist, it is created at version 1.
- Author is set via `llmd config author "name"`, not as a flag.
