// llm.go provides a quick command reference for AI agents.
// Agents call "llmd llm" to get oriented with available commands.

package cli

import "github.com/jpl-au/llmd/sdk"

func llm(ctx sdk.Context, args []string) (sdk.Response, error) {
	return sdk.Text(llmRef), nil
}

var llmRef = `llmd quick reference

READ:    llmd cat <path>                     Read document
         llmd cat --version 3 <path>         Read specific version
         llmd cat -n <path>                  With line numbers

WRITE:   echo "content" | llmd write <path>  Create/update document
         llmd edit <path> <old> <new>        Search and replace
         llmd sed 's/old/new/' <path>        sed-style edit

SEARCH:  llmd grep <pattern> [prefix]        Full-text search
         llmd find <query> [prefix]          Search (paths only)
         llmd glob "docs/*.md"               Path pattern match
         llmd ls [prefix]                    List documents

DELETE:  llmd rm <path>                      Soft-delete (recoverable)
         llmd restore <path>                 Recover deleted doc
         llmd vacuum                         Permanently purge deleted

HISTORY: llmd history <path>                 Version log
         llmd diff <path>                    Diff against previous version
         llmd diff <path>:1 <path>:2         Diff two versions
         llmd revert <path> <version>        Revert to old version

TAGS:    llmd tag <path> <name>              Add tag
         llmd tag <path>                     List tags
         llmd tag -f <name>                  Find docs by tag

LINKS:   llmd link <from> <to>               Link documents
         llmd link <path>                    List links
         llmd unlink <from> <to>             Remove link

BULK:    llmd import <dir>                   Import .md files
         llmd export <prefix> <dir>          Export to filesystem

Use "llmd guide" for full documentation.
Use "llmd guide <topic>" for details (edit, grep, tag, link, import, export).`
