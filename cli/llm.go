// llm.go provides a quick command reference for AI agents.
// Agents call "llmd llm" to get oriented with available commands.

package cli

import "github.com/jpl-au/llmd/sdk"

func llm(ctx sdk.Context, args []string) (sdk.Response, error) {
	return sdk.Text(llmRef), nil
}

var llmRef = `llmd — versioned document store

Documents have paths (notes/readme, projects/spec), full version history,
tags, and links. All content is plain text.

## Self-help

Call the "guide" tool for detailed help on any topic:

  guide                        overview and all commands
  guide <topic>                detailed help (cat, grep, edit, tag, link,
                               workflow, import, export, config, mcp, ...)

## MCP tools

If you are connected via MCP, use tools directly. Three tools are renamed
to avoid collisions with common tool names:

  grep → llmd_grep    find → llmd_find    glob → llmd_glob

All other tool names match their command name (cat, write, edit, ls, etc.).

All tools accept: {"args": [...], "content": "..."}
Use content for document bodies (write, edit). Use args for everything else.

## Commands

READ     cat <path>                  read document
         cat -n <path>               with line numbers
         cat --version N <path>      specific version
         ls [prefix]                 list documents

WRITE    write <path>                create/update (body via content/stdin)
         edit <path> <old> <new>     search and replace
         sed 's/old/new/' <path>     sed-style substitution

SEARCH   grep <pattern> [prefix]     full-text search
         find <query> [prefix]       search (paths only)
         glob "docs/*.md"            path pattern match

ORGANISE tag <path> <name>           add tag
         tag -f <name>               find docs by tag
         link <from> <to>            link documents
         link <path>                 list links

HISTORY  history <path>              version log
         diff <path>                 diff against previous
         diff <path>:1 <path>:2      compare two versions
         revert <path> <version>     restore old version

DELETE   rm <path>                   soft-delete
         restore <path>              recover deleted

TASKS    task list                   board view (all columns)
         task show <id>              metadata + spec body
         task add <title>            create task (body via content/stdin)
         task move <id> <column>     move to column
         task set <id> --flag hold   set flag, priority, assign, etc.
         task rm <id>                soft-delete task (doc untouched)

## Configuration

Author must be set before any write operation:

  config author "Name"`
