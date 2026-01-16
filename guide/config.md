# llmd config

View or set configuration values.

## Usage

```bash
llmd config                     # show config
llmd config <key>               # get value
llmd config <key> <value>       # set value
llmd config --local <key> <value>  # set in local config
```

## Flags

| Flag | Description |
|------|-------------|
| `--local` | Use local config (.llmd/config.yaml) |

## Keys

| Key | Description | Default |
|-----|-------------|---------|
| `author.name` | Default author name | - |
| `author.email` | Default author email | - |
| `sync.files` | Mirror documents to `.llmd/*.md` for @ syntax | `false` |
| `limits.max_path` | Maximum document path length in bytes | `1024` |
| `limits.max_content` | Maximum document content size in bytes | `104857600` (100 MB) |
| `limits.max_line_length` | Maximum line length for scanning in bytes | `10485760` (10 MB) |

## Configuration Locations

| Scope | Path | Purpose |
|-------|------|---------|
| Global | `~/.llmd/config.yaml` | User-wide defaults |
| Local | `.llmd/config.yaml` | Repository-specific settings |

**How it works:**
- Uses local config if it exists, otherwise global
- Writes go to the same place reads come from
- Use `--local` to create/write to local config

Note: `llmd init` does not create config. Use `llmd config` to set up configuration as needed. This follows the git model where init only creates the repository structure.

## Examples

```bash
# Show config (local if exists, else global)
llmd config

# Get author name
llmd config author.name

# Set author name (writes to whichever config is in use)
llmd config author.name "Claude"

# Force write to local config
llmd config --local author.name "Claude"

# Enable file mirroring
llmd config sync.files true
```

## Config Files

**Global** (`~/.llmd/config.yaml`):

```yaml
author:
  name: James Lawson
  email: james@example.com
```

**Local** (`.llmd/config.yaml`):

```yaml
author:
  name: Claude
sync:
  files: true
limits:
  max_path: 2048
  max_content: 209715200  # 200 MB
```

## Notes

- Config is not created by `llmd init` - use `llmd config` to set values
- Global config (`~/.llmd/config.yaml`) is user-wide
- Local config (`.llmd/config.yaml`) is per-repository
- Can be overridden per-command with `-a` flag
- LLMs should use `-a` flag, not change config

## File Sync

`sync.files` mirrors documents to `.llmd/*.md` files for `@` syntax support. Disabled by default.

```bash
llmd config sync.files true   # enable
llmd config sync.files false  # disable
```

When disabled, use `llmd glob` and `llmd cat` to access documents.

## Size Limits

Configure maximum path length and content size to suit your needs.

```bash
# Increase max path length to 2048 bytes
llmd config limits.max_path 2048

# Increase max content to 200 MB
llmd config limits.max_content 209715200

# Check current limits
llmd config limits.max_path
llmd config limits.max_content
```

Defaults are 1024 bytes for paths and 100 MB for content.
