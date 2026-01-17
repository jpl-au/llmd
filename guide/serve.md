# llmd serve

Start an MCP (Model Context Protocol) server over stdio.

## Usage

```bash
llmd serve              # serve default database (llmd.db)
llmd serve --db docs    # serve specific database (llmd-docs.db)
```

## Description

The `serve` command starts an MCP server that exposes llmd's document store to any MCP-compatible LLM client. The server communicates over stdio using JSON-RPC.

## MCP Client Configuration

### Claude Code / Claude Desktop

Add to your MCP settings:

```json
{
  "mcpServers": {
    "llmd": {
      "command": "llmd",
      "args": ["serve"],
      "cwd": "/path/to/your/project"
    }
  }
}
```

For a specific database:

```json
{
  "mcpServers": {
    "llmd-docs": {
      "command": "llmd",
      "args": ["serve", "--db", "docs"],
      "cwd": "/path/to/your/project"
    }
  }
}
```

Using environment variables (alternative to args):

```json
{
  "mcpServers": {
    "llmd-docs": {
      "command": "llmd",
      "args": ["serve"],
      "env": {
        "LLMD_DB": "docs",
        "LLMD_DIR": "/path/to/your/project/.llmd"
      }
    }
  }
}
```

### Other MCP Clients

Configure your client to spawn `llmd serve` in the directory containing your `.llmd` store. Use `--db` to serve a specific database.

## Resources

MCP resources provide read-only access to documents:

| URI Pattern | Description |
|-------------|-------------|
| `llmd://documents/{path}` | Read document content |
| `llmd://documents/{path}/v/{version}` | Read specific version |

## Tools

MCP tools provide full document operations:

| Tool | Description |
|------|-------------|
| `llmd_list` | List documents |
| `llmd_read` | Read document content |
| `llmd_write` | Create or update document |
| `llmd_delete` | Soft delete document |
| `llmd_restore` | Restore deleted document |
| `llmd_move` | Move/rename document |
| `llmd_search` | Full-text search (FTS5) |
| `llmd_grep` | Regex pattern search |
| `llmd_history` | Get version history |
| `llmd_diff` | Show differences between versions |
| `llmd_edit` | Edit via search/replace |
| `llmd_sed` | Edit via sed-style substitution |
| `llmd_glob` | List paths matching a pattern |
| `llmd_tag_add` | Add a tag to a document |
| `llmd_tag_remove` | Remove a tag from a document |
| `llmd_tags` | List tags |
| `llmd_link` | Create or list document links |
| `llmd_unlink` | Remove a link |
| `llmd_import` | Import files from filesystem |
| `llmd_export` | Export documents to filesystem |
| `llmd_sync` | Sync filesystem changes to database |
| `llmd_config_get` | Get configuration value |
| `llmd_config_set` | Set configuration value |
| `llmd_guide` | Get help/guide content |

### Tool Parameters

#### llmd_list

| Parameter | Required | Description |
|-----------|----------|-------------|
| `prefix` | No | Filter by path prefix |
| `include_deleted` | No | Include soft-deleted documents |
| `deleted_only` | No | Show only deleted documents |

#### llmd_read

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `version` | No | Specific version (default: latest) |
| `include_deleted` | No | Allow reading deleted documents |

#### llmd_write

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path |
| `content` | Yes | Document content |
| `author` | Yes | Author attribution |
| `message` | No | Version message |

#### llmd_delete

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `version` | No | Delete only this specific version |

#### llmd_restore

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |

#### llmd_move

| Parameter | Required | Description |
|-----------|----------|-------------|
| `from` | Yes | Source path |
| `to` | Yes | Destination path |

#### llmd_search

| Parameter | Required | Description |
|-----------|----------|-------------|
| `query` | Yes | Search query |
| `prefix` | No | Limit to path prefix |
| `include_deleted` | No | Include deleted documents |
| `deleted_only` | No | Search only deleted |

#### llmd_history

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `limit` | No | Max versions to return |
| `include_deleted` | No | Include deleted versions |

#### llmd_diff

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `path2` | No | Second document (for comparing two documents) |
| `version1` | No | First version to compare |
| `version2` | No | Second version to compare |
| `include_deleted` | No | Allow diffing deleted documents |

#### llmd_edit

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `old` | Yes | Text to find |
| `new` | No | Text to replace with |
| `author` | Yes | Author attribution |
| `message` | No | Version message |

#### llmd_glob

| Parameter | Required | Description |
|-----------|----------|-------------|
| `pattern` | No | Glob pattern (supports *, **, ?) |

#### llmd_grep

| Parameter | Required | Description |
|-----------|----------|-------------|
| `pattern` | Yes | Regex pattern |
| `path` | No | Limit to path prefix |
| `ignore_case` | No | Case insensitive search |
| `paths_only` | No | Only return matching paths |
| `include_deleted` | No | Include deleted documents |
| `deleted_only` | No | Search only deleted documents |

#### llmd_sed

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `expression` | Yes | Sed expression (e.g., s/old/new/ or s/old/new/g) |
| `author` | Yes | Author attribution |
| `message` | No | Version message |

#### llmd_tag_add

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `tag` | Yes | Tag to add |

#### llmd_tag_remove

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path or 8-character key |
| `tag` | Yes | Tag to remove |

#### llmd_tags

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | No | Document path or 8-character key (list all if empty) |

#### llmd_link

| Parameter | Required | Description |
|-----------|----------|-------------|
| `from` | No | Source document path (required for creating) |
| `to` | No | Target document path (required for creating) |
| `tag` | No | Link tag for categorisation |
| `list` | No | List links for 'from' path |
| `orphan` | No | List documents with no links |

#### llmd_unlink

| Parameter | Required | Description |
|-----------|----------|-------------|
| `id` | No | Link ID to remove |
| `tag` | No | Remove all links with this tag |

#### llmd_import

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Filesystem path to import from |
| `prefix` | No | Target path prefix in store |
| `author` | Yes | Author attribution |
| `flat` | No | Flatten directory structure |
| `hidden` | No | Include hidden files/directories |
| `dry_run` | No | Show what would be imported |

#### llmd_export

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | Yes | Document path, 8-character key, or prefix (ending with /) |
| `dest` | Yes | Filesystem destination path |
| `version` | No | Export specific version |
| `force` | No | Overwrite existing files |

Examples:
- `path: "docs/readme"` - exports single document by path
- `path: "a1b2c3d4"` - exports single document by key
- `path: "docs/"` - exports all documents under `docs/` prefix

#### llmd_sync

| Parameter | Required | Description |
|-----------|----------|-------------|
| `author` | Yes | Author attribution |
| `dry_run` | No | Show what would be synced |
| `message` | No | Commit message for synced documents |

#### llmd_config_get

| Parameter | Required | Description |
|-----------|----------|-------------|
| `key` | No | Config key or empty for all |

#### llmd_config_set

| Parameter | Required | Description |
|-----------|----------|-------------|
| `key` | Yes | Config key |
| `value` | Yes | Value to set |

#### llmd_guide

| Parameter | Required | Description |
|-----------|----------|-------------|
| `topic` | No | Guide topic or empty for index |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `LLMD_DB` | Database name (equivalent to `--db`) |
| `LLMD_DIR` | Database directory (skip discovery) |

These are particularly useful for MCP client configuration where you can set them in the `env` block instead of using command-line arguments.

## Notes

- The server must be started in a directory with an initialised llmd store
- All soft deletions are recoverable via `llmd_restore`
- The `vacuum` command is intentionally excluded for safety (use CLI)
- Author is required for all write operations to ensure proper attribution
