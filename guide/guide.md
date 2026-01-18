# llmd Guide

A document store that declutters your filesystem. Stores documents in `.llmd/llmd.db` with versioning, search, and history.

## Quick Start

```bash
llmd init                                 # initialise store
echo "# Hello" | llmd write docs/readme   # create document
llmd cat docs/readme                      # read document
llmd ls                                   # list documents
```

## Commands

Run `llmd guide <command>` for detailed help on any command.

| Command | Description |
|---------|-------------|
| `init` | Initialise a new store |
| `config` | View or set configuration |
| `ls` | List documents |
| `cat` | Read a document |
| `write` | Write stdin to a document |
| `edit` | Edit via search/replace or line range |
| `sed` | Stream editor (sed-style substitution) |
| `grep` | Search using regex |
| `find` | Full-text search (FTS5) |
| `rm` | Soft delete a document |
| `restore` | Restore a deleted document |
| `mv` | Move/rename a document |
| `tag` | Manage document tags |
| `link` | Create links between documents |
| `unlink` | Remove links between documents |
| `glob` | List paths matching a pattern |
| `history` | Show version history |
| `diff` | Compare document versions |
| `revert` | Revert to a previous version |
| `import` | Bulk import from filesystem |
| `export` | Export to filesystem |
| `sync` | Sync filesystem changes to database |
| `vacuum` | Permanently delete soft-deleted docs |
| `serve` | Start MCP server for LLM integration |
| `llm` | Getting started guide for LLMs |

## Command Usage

### Read & List

```bash
llmd cat docs/readme                   # read document
llmd cat docs/readme -v 3              # read version 3
llmd cat docs/readme -o json           # with metadata
llmd ls                                # list all
llmd ls docs/ -t                       # tree view
llmd glob "docs/**"                    # glob pattern
```

### Write & Edit

```bash
echo "content" | llmd write docs/readme
llmd write docs/readme < file.md
llmd edit docs/readme "old" "new"         # search/replace
llmd edit docs/readme -l 5:10 < new.txt   # replace lines
llmd sed -i 's/old/new/' docs/readme      # sed-style
```

### Search

```bash
# Regex search (like Unix grep)
llmd grep "TODO" docs/                 # basic pattern
llmd grep "error|warning" docs/        # alternation
llmd grep -i "auth.*token"             # case-insensitive
llmd grep -v "TODO"                    # invert match (non-matching lines)
llmd grep -l "error"                   # list matching paths only

# Full-text search (FTS5)
llmd find "authentication"             # word search
llmd find "error OR warning"           # boolean operators
llmd find "auth*"                      # prefix matching
llmd find "TODO" -p docs/              # scope to path
```

### History & Versions

```bash
llmd history docs/readme               # show versions
llmd history docs/readme -n 5          # last 5
llmd cat docs/readme -v 3              # read version 3
llmd diff docs/readme                  # diff latest vs previous
llmd diff docs/readme -v 1:3           # diff versions 1 and 3
llmd revert docs/readme 3              # revert to version 3
llmd revert abc12345                   # revert using key from history
```

### Delete & Restore

```bash
llmd rm docs/readme                    # soft delete
llmd ls -D                             # list deleted
llmd restore docs/readme               # restore
llmd vacuum                            # permanently delete
```

### Move & Organise

```bash
llmd mv old/path new/path              # rename/move
llmd import ./docs/                    # import from filesystem
llmd export docs/ ./output/            # export to filesystem
```

## For LLMs

Always use `-a` to identify yourself:

```bash
echo "content" | llmd write docs/readme -a "claude-code"
llmd edit docs/readme "old" "new" -a "claude-code" -m "Fixed typo"
```

Use `-o json` for structured output:

```bash
llmd ls -o json
llmd cat docs/readme -o json
llmd find "auth" -o json
```

### Writing Multi-Line Documents

When writing documents using heredocs, use `LLMD_DOC` as the delimiter to avoid conflicts with code examples in your content:

```bash
llmd write docs/readme -a "claude-code" -m "Initial draft" << 'LLMD_DOC'
# My Document

Here is some content with code examples:

```bash
# This nested heredoc won't cause issues
cat << 'EOF'
example content
EOF
```

The document continues...
LLMD_DOC
```

**Why `LLMD_DOC`?** Standard delimiters like `EOF` may appear in code examples within your document, causing shell parsing errors. Using `LLMD_DOC` (or another unique delimiter) prevents nested heredoc conflicts.

**Alternative approach:** Write to a temporary file first, then pipe:

```bash
# Write content to temp file (avoids heredoc issues entirely)
cat > /tmp/doc.md << 'EOF'
# Content with any heredoc examples
EOF

# Then import to llmd
llmd write docs/readme -a "claude-code" < /tmp/doc.md
```

## Global Flags

| Flag | Description |
|------|-------------|
| `-a, --author` | Version attribution |
| `-m, --message` | Version message |
| `-o, --output` | Output format: `json` |
| `--force` | Skip confirmations |
| `--db` | Database name (selects llmd-{name}.db) |
| `--dir` | Database directory (skip discovery) |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `LLMD_DB` | Default database name (equivalent to `--db`) |
| `LLMD_DIR` | Default database directory (equivalent to `--dir`) |

Priority: flags override environment variables.

```bash
# Using flags
llmd --db docs ls

# Using environment variables
export LLMD_DB=docs
llmd ls

# Or inline
LLMD_DB=docs llmd ls
LLMD_DIR=/path/to/.llmd llmd ls
```

## Document Paths

- Omit `.md` extension (recommended): `docs/readme` not `docs/readme.md`
- Forward slashes: `docs/api/auth`
- No leading slash: `docs/readme` not `/docs/readme`
