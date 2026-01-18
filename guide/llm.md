# llmd llm

Getting started guide for LLMs to discover available commands.

## Usage

```bash
llmd llm
```

## Description

Outputs a quick reference of available commands for LLM integration. This helps LLMs discover what operations are available without reading the full guide.

## MCP

The easiest way for an LLM to get started is through the MCP server. Use
`llmd guide serve` to understand how to use commands.

## Output

```
Commands work like standard filesystem/unix tools:
  ls, cat, rm, mv, grep, find, sed, diff, history

Additional commands:
  write     Write stdin to document
  edit      Search/replace or line-range edit
  tag       Manage document tags
  glob      List paths matching pattern
  restore   Restore deleted document
  import    Import from filesystem
  export    Export to filesystem
  sync      Sync filesystem changes to store
  serve     Start MCP server

Always use -a for author attribution on writes:
  echo "content" | llmd write path -a "claude-code"
  llmd edit path "old" "new" -a "claude-code"

Getting help:
  llmd guide                # full documentation
  llmd guide <command>      # help for specific command
  llmd guide serve          # MCP server setup and tools
  llmd guide workflow       # Common workflow patterns are described here

Using via MCP (Model Context Protocol):
  - If tools return "store not initialised", call llmd_init first
  - All write tools require 'author' parameter
  - Run 'llmd guide serve' for full MCP tool reference
```