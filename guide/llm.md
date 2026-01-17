# llmd llm

Show LLM documentation hints and command discovery.

## Usage

```bash
llmd llm
```

## Description

Outputs a quick reference of available commands for LLM integration. This helps LLMs discover what operations are available without reading the full guide.

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

Use 'llmd guide' for full documentation.
Use 'llmd guide <command>' for command-specific help.
```

## Notes

- Use `llmd guide` for comprehensive documentation
- Use `llmd guide <command>` for detailed help on specific commands
- Via MCP: if tools return "store not initialised", call `llmd_init` first
