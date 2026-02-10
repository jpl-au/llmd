// guide.go provides built-in documentation for humans and LLMs.
//
// Usage:
//
//	llmd guide              Full command guide
//	llmd guide <topic>      Help on a specific topic (e.g. "edit", "grep")

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func guide(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return sdk.Text(guideOverview), nil
	}

	topic := args[0]
	if text, ok := guideTopic[topic]; ok {
		return sdk.Text(text), nil
	}
	return nil, fmt.Errorf("guide: unknown topic: %s\n\nAvailable: %s",
		topic, strings.Join(guideTopics(), ", "))
}

func guideTopics() []string {
	topics := make([]string, 0, len(guideTopic))
	for k := range guideTopic {
		topics = append(topics, k)
	}
	return topics
}

var guideOverview = `llmd - a document store for LLMs and humans

DOCUMENT OPERATIONS
  cat <path>              Read a document (-n for line numbers, --version N for specific version)
  write <path>            Write stdin to a document (pipe content in)
  edit <path> <old> <new> Search and replace text in a document
  sed 's/old/new/' <path> sed-style substitution
  rm <path>               Soft-delete a document (recoverable)
  mv <from> <to>          Move or rename a document
  restore <path>          Restore a soft-deleted document

SEARCH & DISCOVERY
  ls [prefix]             List documents (-l long, -a include deleted, -t sort by time)
  grep <pattern> [path]   Full-text search (-n line numbers, -l files only, -c count)
  find <query> [path]     Search and return matching paths
  glob <pattern>          Match paths with shell-style patterns (*, **, ?)

VERSION CONTROL
  history <path>          Show version history (-n limit)
  diff <a> [b]            Compare versions (use path:version format)
  revert <path> <version> Revert to a previous version

TAGS & LINKS
  tag <path> <name>       Add a tag to a document
  tag -d <path> <name>    Remove a tag
  tag <path>              List tags on a document
  tag -f <name>           Find documents with a tag
  link <from> <to>        Create a link between documents
  unlink <from> <to>      Remove a link

BULK OPERATIONS
  import <dir>            Import .md files from filesystem (--prefix, --dry-run, --force)
  export <prefix> <dir>   Export documents to filesystem (--overwrite)

ADMIN
  init                    Initialize a new store
  config [key] [value]    View or set configuration
  vacuum                  Permanently delete soft-deleted documents
  mcp / serve             Start MCP stdio server
  plugins                 List loaded plugins
  version                 Show version

Use "llmd guide <topic>" for details on a specific command.
Use "llmd <command> --help" for flag reference.`

var guideTopic = map[string]string{
	"edit": `EDITING DOCUMENTS

Search and replace:
  llmd edit <path> <old> <new>
  echo "new content" | llmd write <path>

sed-style substitution:
  llmd sed 's/old/new/' <path>
  llmd sed 's|old|new|' <path>    (any delimiter)

The edit command replaces the first occurrence of <old> with <new> and
creates a new version. Use write to replace entire document content.`,

	"grep": `SEARCHING DOCUMENTS

Full-text search (FTS5 syntax):
  llmd grep <pattern>              Search all documents
  llmd grep <pattern> docs/        Search under prefix
  llmd grep -n <pattern>           Show line numbers
  llmd grep -l <pattern>           Show only matching paths
  llmd grep -c <pattern>           Show match counts
  llmd grep -C3 <pattern>          Show 3 lines of context

Path-only search:
  llmd find <query>                Returns just document paths

FTS5 supports: phrases ("hello world"), prefix (hello*), boolean (a OR b),
column filters, NEAR(a b).`,

	"tag": `TAGGING DOCUMENTS

  llmd tag <path> <name>           Add a tag
  llmd tag -d <path> <name>        Remove a tag
  llmd tag <path>                  List tags on a document
  llmd tag                         List all tags with counts
  llmd tag -f <name>               Find documents with a tag

Tag names must be lowercase alphanumeric with hyphens, 1-64 characters.`,

	"link": `LINKING DOCUMENTS

  llmd link <from> <to>                     Create a link
  llmd link --label depends-on <from> <to>  Create a labeled link
  llmd link <path>                          List outgoing links
  llmd link --in <path>                     List incoming links
  llmd unlink <from> <to>                   Remove a link

Links are directional. Use labels to describe the relationship.`,

	"import": `IMPORTING DOCUMENTS

  llmd import <dir>                Import .md files from directory
  llmd import --prefix docs/ <dir> Import under a path prefix
  llmd import --dry-run <dir>      Preview without importing
  llmd import --force <dir>        Re-import even if unchanged

Only .md files are imported. Existing documents are updated only if
content has changed (unless --force is used).`,

	"export": `EXPORTING DOCUMENTS

  llmd export <prefix> <dir>             Export documents to directory
  llmd export --overwrite <prefix> <dir> Overwrite existing files

Documents are written as .md files preserving path structure.`,
}
