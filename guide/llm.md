# Getting Started with llmd

Quick reference for LLMs to use llmd effectively.

## Recommended: Use MCP

If your environment supports MCP (Model Context Protocol), that's the better way to use llmd. MCP provides native tool integration without shell command overhead.

**Check if MCP is available:** Look for `llmd_*` tools (like `llmd_list`, `llmd_read`, `llmd_write`) in your available tools. If present, use those instead of CLI commands.

**Setting up MCP:** Ask the user to configure llmd as an MCP server. See `llmd guide serve` for setup instructions.

**Using MCP tools:**
- If tools return "store not initialised", call `llmd_init` first
- All write tools require `author` parameter (e.g., `author: "claude-code"`)
- See `llmd guide serve` for full tool reference

---

## Using the CLI

If MCP is not available, you can use llmd via command line.

### First Steps

```bash
llmd ls                    # Check if store exists
llmd init                  # Initialise if needed (only once)
```

### Author Attribution

**Always use `-a` on write operations** (optional in CLI, but recommended for audit trail):

```bash
echo "content" | llmd write docs/notes -a "claude-code"
llmd edit docs/notes "old" "new" -a "claude-code"
llmd sed -i 's/foo/bar/' docs/notes -a "claude-code"
```

Without `-a`, changes cannot be attributed to you in history.

### Commands

Commands work like standard filesystem/unix tools:

| Command | Purpose |
|---------|---------|
| `ls` | List documents |
| `cat` | Read a document |
| `write` | Create/update document (stdin) |
| `edit` | Search/replace or line-range edit |
| `rm` | Soft delete |
| `mv` | Move/rename |
| `grep` | Regex search |
| `find` | Full-text search |
| `sed` | Sed-style substitution |
| `diff` | Compare versions |
| `history` | Show version history |
| `restore` | Restore deleted document |
| `tag` | Manage document tags |

### Common Patterns

```bash
# Read a document
llmd cat docs/readme

# Write new content
echo "# Title" | llmd write docs/readme -a "claude-code"

# Edit existing content
llmd edit docs/readme "old text" "new text" -a "claude-code"

# Search for content
llmd grep "TODO" docs/
llmd find "authentication"

# View history and versions
llmd history docs/readme
llmd cat docs/readme -v 3        # read version 3
llmd diff docs/readme -v 1:3     # compare versions
```

### Document Paths

- Omit `.md` extension (recommended): `docs/readme` not `docs/readme.md`
- Forward slashes: `docs/api/auth`
- No leading slash: `docs/readme` not `/docs/readme`

### More Help

```bash
llmd guide                  # Full documentation
llmd guide <command>        # Help for specific command
llmd guide workflow         # Workflow patterns and best practices
```
