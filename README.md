# llmd

AI coding agents leave markdown files everywhere. Context docs, specs, notes,
half-finished thoughts. After a few months your filesystem is a graveyard of
`.md` files, half of them are outdated, and your `CLAUDE.md` hits 55KB while
your context sessions keep getting shorter.

llmd is a document store for LLMs and humans. It was built for LLMs in
orchestration between Claude Code, Gemini CLI and Antigravity. It's designed
as a context database specifically for AI agent usage that can be easily
shared and committed to a repo. It also includes a task board with git
branch tracking, so agents and humans can coordinate work in the same place
they store context.

It works well as a way to easily provide customised documentation for LLMs
for any project. And because it's using SQLite3, your docs are easily
accessible even without llmd: just use SQLite3's command line interface.

## How it helps

**llmd** is a SQLite-backed document store where both you and your AI agents
write context. Everything is versioned automatically, searchable, and
organised in a single database file. Commit it to git and share context
across sessions. Agents interact via the **MCP server** — native integration with
Claude Code, Cursor, and other MCP clients. For humans (and agents that
prefer it), the CLI mirrors standard Unix commands — `cat`, `ls`, `grep`,
`sed`.

## Key Features

- **Auto-versioning** — Every write creates a new version. Diff and revert anytime.
- **Author tracking** — See what the LLM changed vs what you changed.
- **Full-text search** — Search with `llmd find` or `llmd grep`.
- **Soft delete** — Nothing is ever lost. Restore with `llmd restore`.
- **Task board** — Track work with columns, priorities, and git branch integration.
- **MCP server** — Native integration with Claude Code, Cursor, and other MCP clients.

### llmd teaches itself to your agents

Get your agent to run `llmd llm` for a quick command reference — agents
should hopefully naturally gravitate to this. For deeper dives, `llmd guide`
provides full documentation with examples and workflows.

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
llmd config author "Your Name"               # set author
echo "# API Docs" | llmd write docs/api      # write a document
llmd cat docs/api                            # read it back
llmd sed -i 's/API/REST API/' docs/api       # edit with sed
llmd grep "API" docs/                        # search
llmd history docs/api                        # see version history
llmd llm                                     # quick command reference
```

## Commands

```
Reading:    cat, ls (--tree), grep, find, glob
Writing:    write, edit, sed, rm, mv, restore, revert
History:    history, diff
Tags:       tag, link, unlink
Tasks:      task, status, review
Bulk:       import, export, mirror
Admin:      init, config, vacuum, version, mcp, serve, plugins
Help:       guide, llm
```

Run `llmd <command> --help` for usage, or `llmd guide <topic>` for
detailed documentation. Agents can run `llmd llm` for a quick reference.

## MCP Server

For Claude Code, Cursor, or other MCP clients:

```json
{
  "mcpServers": {
    "llmd": {
      "command": "llmd",
      "args": ["mcp"]
    }
  }
}
```

Use `--db` to connect to a specific database:

```json
{
  "mcpServers": {
    "llmd-docs": {
      "command": "llmd",
      "args": ["--db", "docs", "mcp"]
    }
  }
}
```

You can run multiple llmd servers for different databases simultaneously.

## Storage

All data lives in a `.llmd/` directory. By default, llmd searches upward from
the current directory to find it (like git finds `.git/`).

Documents are stored in `.llmd/llmd.db`. You can have multiple databases
(e.g., `llmd-docs.db`, `llmd-notes.db`) and switch between them with `--db`:

```bash
llmd init --db docs       # create llmd-docs.db
llmd ls --db docs         # use it
```

## Documentation

Full documentation is available via `llmd guide` or browse the
[guide/](guide/) directory.

## Acknowledgements

### Built with

- [Go](https://go.dev) — Programming language (BSD 3-Clause)
- [Claude Code](https://claude.ai/claude-code) — AI coding agent by Anthropic
- [Gemini CLI](https://github.com/google-gemini/gemini-cli) — AI coding agent by Google
- [Antigravity](https://antigravity.dev) — AI coding agent

### Libraries

| Library | Description | Licence |
|---------|-------------|---------|
| [modernc.org/sqlite](https://modernc.org/sqlite) | Pure Go SQLite | BSD 3-Clause |
| [go-sdk](https://github.com/modelcontextprotocol/go-sdk) | MCP protocol | MIT |
| [yaegi](https://github.com/traefik/yaegi) | Go interpreter (plugins) | Apache 2.0 |
| [glamour](https://github.com/charmbracelet/glamour) | Terminal markdown rendering | MIT |
| [lipgloss v2](https://charm.land/lipgloss/v2) | Terminal styling and tree rendering | MIT |
| [go-udiff](https://github.com/aymanbagabas/go-udiff) | Unified diff | MIT |
| [xxh3](https://github.com/zeebo/xxh3) | Content hashing | BSD 2-Clause |

## Licence

BSL 1.1 — free for all use except commercial distribution as a bundled
product or hosted service. See [LICENSE](LICENSE).
