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

## Key Features

- **Auto-versioning** - Every write creates a new version. Diff and revert anytime.
- **Author tracking** - See what the LLM changed vs what you changed.
- **Full-text search** - Search with `llmd find` or `llmd grep`.
- **Soft delete** - Nothing is ever lost. Restore with `llmd restore`.
- **Task board** - Track work with columns, priorities, and git branch integration. Agents claim work with `task next`.
- **Audit threads** - Agent-to-agent and human-to-agent review threads on documents and tasks.
- **MCP server** - Native integration with Claude Code, Cursor, and other MCP clients.
- **HTTP API** - REST interface for programmatic access.

### llmd teaches itself to your agents

`llmd guide` provides full documentation with examples and workflows.
AI agents are prompted to start there. Every guide page puts the rules
and the syntax first and the explanation after, so a model that reads
only the first screen still gets the call right. When output is piped,
as it is for an agent in a shell, `llmd <command> --help` prints the
same short form the MCP tool list carries, and tables come out as
tab-separated lines.

```bash
llmd guide              # Full guide with all commands
llmd guide edit         # Learn search/replace and line-range editing
llmd guide claude-code  # Claude Code integration patterns
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
```

## Commands

```
Reading:    cat, ls (--tree), grep, find, glob
Writing:    write, edit, sed, rm, mv, restore, revert
History:    history, diff
Tags:       tag, link, unlink
Tasks:      task, status, review
Audits:     audit (add, reply, list, show, resolve, rm, restore, status)
Bulk:       import, export, mirror
Admin:      init, config, vacuum, version, mcp, serve, extensions
Help:       guide
```

Run `llmd <command> --help` for usage, or `llmd guide <topic>` for
detailed documentation on any command. See `llmd guide mcp` for MCP
integration, `llmd guide serve` for the HTTP API, and
`llmd guide claude-code` for Claude Code integration patterns.

## MCP Server

Add llmd as an MCP server in your editor config:

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

See `llmd guide mcp` for details on tool naming, input schema, and
author attribution.

## Acknowledgements

### Built with

- [Go](https://go.dev) - Programming language (BSD 3-Clause)
- [Claude Code](https://claude.ai/claude-code) - AI coding agent by Anthropic
- [Gemini CLI](https://github.com/google-gemini/gemini-cli) - AI coding agent by Google
- [Antigravity](https://antigravity.dev) - AI coding agent

### Libraries

| Library | Description | Licence |
|---------|-------------|---------|
| [modernc.org/sqlite](https://modernc.org/sqlite) | Pure Go SQLite | BSD 3-Clause |
| [qwr](https://github.com/jpl-au/qwr) | Serialised writes and pooled reads for SQLite | MIT |
| [chain](https://github.com/jpl-au/chain) | HTTP routing | MIT |
| [http-client](https://github.com/jpl-au/http-client) | Webhook delivery | MIT |
| [go-sdk](https://github.com/modelcontextprotocol/go-sdk) | MCP protocol | MIT |
| [glamour v2](https://charm.land/glamour/v2) | Terminal markdown rendering | MIT |
| [lipgloss v2](https://charm.land/lipgloss/v2) | Terminal styling and tree rendering | MIT |
| [go-udiff](https://github.com/aymanbagabas/go-udiff) | Unified diff | MIT |
| [xxh3](https://github.com/zeebo/xxh3) | Content hashing | BSD 2-Clause |
| [yaml.v3](https://gopkg.in/yaml.v3) | Configuration files | MIT and Apache 2.0 |

## Licence

BSL 1.1 - free for all use except commercial distribution as a bundled
product or hosted service. See [LICENSE](LICENSE).
