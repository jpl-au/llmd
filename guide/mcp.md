# llmd mcp

Start an MCP (Model Context Protocol) server over stdio.

## Usage

```
llmd mcp
llmd serve
```

## Examples

```bash
# Start the MCP server
llmd mcp

# Equivalent alias
llmd serve
```

## Notes

- Communicates over stdin/stdout using the MCP protocol.
- Exposes llmd commands as MCP tools.
- Tool names match command names, except: `grep` becomes `llmd_grep`, `find` becomes `llmd_find`, `glob` becomes `llmd_glob`.
- All tools accept a JSON input schema of the form `{"args": [...], "content": "..."}`.
