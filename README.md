# llmd

AI coding agents leave markdown files everywhere. Context docs, specs, notes, half-finished thoughts. After a few months your filesystem is a graveyard of `.md` files, and half of them are outdated.

Meanwhile, your `CLAUDE.md` hits 55KB and your context sessions keep getting shorter.

llmd is a document store for LLMs and humans. It was built for LLMs in orchestration between Claude Code, Gemini CLI and Antigravity.

It's designed as a context database specifically for AI agent usage that can be easily shared and committed to a repo to allow teams and agents to work on a codebase.

It also works well as a way to easily provide customised documentation for LLMs for any project. And because it's using SQLite3, your docs are easily accessible even without llmd: just use SQLite3's command line interface.

## How it helps

**llmd** is a SQLite-backed document store where both you and your AI agents write context. Everything is versioned automatically, searchable, and organised in a single database file. Commit it to git and share context with your team.

Agents interact via the **MCP server** - native integration with Claude Code, Cursor, and other MCP clients. No shell commands needed. For humans (and agents that prefer it), the CLI mirrors standard unix commands - `cat`, `ls`, `grep`, `sed`.

## Key Features

- **Auto-versioning** - Every write creates a new version. Diff and revert anytime.
- **Author tracking** - See what the LLM changed vs what you changed.
- **Full-text search** - Search with `llmd find` or `llmd grep`.
- **Soft delete** - Nothing is ever lost. Restore with `llmd restore`.
- **MCP server** - Native integration with Claude Code, Cursor, and other MCP clients.

### llmd teaches itself to your agents

Get your agent to run `llmd llm` for a quick command reference - agents should hopefully naturally gravitate to this. For deeper dives, `llmd guide` provides full documentation. The embedded guide helps teach and provides quick reference when stuck.

```bash
llmd llm                # Quick command reference (agents start here)
llmd guide              # Full guide with all commands
llmd guide edit         # Learn search/replace and line-range editing
```

## Install

```bash
go install github.com/jpl-au/llmd@latest
```

Or download from [GitHub Releases](https://github.com/jpl-au/llmd/releases).

## Quickstart

```bash
llmd init                                    # initialise store
echo "# API Docs" | llmd write docs/api      # write a document
llmd cat docs/api                            # read it back
llmd sed -i 's/API/REST API/' docs/api       # edit with sed
llmd grep "API" docs/                        # search
llmd history docs/api                        # see version history
llmd llm                                     # quick command reference
```

## Commands

Familiar filesystem commands with superpowers:

| Command | Description |
|---------|-------------|
| `init` | Initialise a new llmd store |
| `cat` | Read a document (`-n` lines, `-l` range) |
| `ls` | List documents (`-l` for long format) |
| `write` | Write stdin to a document |
| `edit` | Search/replace or line range edit |
| `sed` | sed-style substitution (`-i 's/old/new/'`) |
| `grep` | Search (`-C` context, `-v` invert, `-c` count) |
| `find` | Full-text search |
| `glob` | List paths matching a pattern |
| `rm` | Soft delete (`-r` for recursive) |
| `mv` | Move/rename |
| `history` | Version history |
| `diff` | Compare document versions |
| `revert` | Revert to a previous version of a document |
| `restore` | Restore deleted documents |
| `vacuum` | Permanently delete soft-deleted docs |
| `tag` | Manage document tags |
| `link` | Create links between documents |
| `unlink` | Remove document links |
| `import` | Bulk import from filesystem |
| `export` | Export documents to filesystem |
| `sync` | Sync filesystem changes back to db |
| `db` | List/manage databases |
| `config` | View or set configuration |
| `guide` | Built-in help (LLM-friendly) |
| `llm` | Quick command reference for LLMs |
| `serve` | Start MCP server |
| `version` | Show version information |

## MCP Server

For Claude Code, Cursor, or other MCP clients:

```json
{
  "mcpServers": {
    "llmd": {
      "command": "llmd",
      "args": ["serve"]
    }
  }
}
```

To connect to a specific database or directory, pass `--db` or `--dir`:

```json
{
  "mcpServers": {
    "llmd-docs": {
      "command": "llmd",
      "args": ["--db", "docs", "serve"]
    },
    "llmd-notes": {
      "command": "llmd",
      "args": ["--dir", "/path/to/project/.llmd", "--db", "notes", "serve"]
    }
  }
}
```

You can run multiple llmd servers for different databases simultaneously.

## Storage

All data lives in a `.llmd/` directory. By default, llmd searches upward from the current directory to find it (like git finds `.git/`).

**Database** - Documents are stored in `.llmd/llmd.db`. You can have multiple databases (e.g., `llmd-docs.db`, `llmd-notes.db`) and switch between them with `--db`:

```bash
llmd init --db docs           # Create llmd-docs.db
llmd ls --db docs             # Use it
export LLMD_DB=docs           # Or set as default
```

**Shared vs Local** - By default, the database is committed to your repo so teams can share documentation. Use `--local` to keep it private (auto-added to `.gitignore`):

```bash
llmd init --local             # Personal notes, not committed
llmd db notes --local         # Mark existing db as local
```

**Explicit directory** - Skip the upward search and specify the `.llmd/` location directly:

```bash
llmd --dir /path/to/.llmd ls  # Use specific store
export LLMD_DIR=/path/to/.llmd
```

**File mirroring** - Enable `sync.files` to mirror documents as `.md` files in `.llmd/`. This lets you use `@` syntax in Claude Code to reference docs, or edit them in your IDE. Changes sync back with `llmd sync`:

```bash
llmd config sync.files true   # Enable mirroring
# Documents now appear as .llmd/docs/api.md, etc.
llmd sync                     # Sync external edits back to db
```

**Config** - Settings live in `.llmd/config.yaml` (per-project) or `~/.llmd/config.yaml` (global fallback).

## Documentation

Full documentation is available via `llmd guide` or browse the [guide/](guide/) directory:

- [Main Guide](guide/guide.md) - Overview and quick reference
- [Command Reference](guide/) - One file per command

## Acknowledgements

### Built with

- [Go](https://go.dev) - Programming language (BSD 3-Clause)
- [Claude Code](https://claude.ai/claude-code) - AI coding agent by Anthropic
- [Gemini CLI](https://github.com/google-gemini/gemini-cli) - AI coding agent by Google
- [Antigravity](https://antigravity.dev) - AI coding agent

### Libraries

| Library | Description | License |
|---------|-------------|---------|
| [modernc.org/sqlite](https://modernc.org/sqlite) | Pure Go SQLite | BSD 3-Clause |
| [mcp-go](https://github.com/mark3labs/mcp-go) | MCP protocol implementation | MIT |
| [cobra](https://github.com/spf13/cobra) | CLI framework | Apache 2.0 |
| [glamour](https://github.com/charmbracelet/glamour) | Terminal markdown rendering | MIT |
| [go-diff](https://github.com/sergi/go-diff) | Diff algorithm | MIT |
| [yaml.v3](https://gopkg.in/yaml.v3) | YAML parsing | MIT / Apache 2.0 |

## License

BSL 1.1 - free for all use except commercial distribution as a bundled product or as a hosted/managed service (SaaS). See [LICENSE](LICENSE) for details.
