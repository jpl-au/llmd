# llmd grep

Full-text search across documents, returning the matching markdown
section of each hit rather than the whole document.

## Usage

```
llmd grep [flags] <query> [<path>]
```

## Flags

Modes are mutually exclusive; `--sections` is the default.

| Flag | Description |
|------|-------------|
| `--sections` | Return the matching markdown section per hit (default) |
| `--lines` | Return the matching line per hit, with optional `-C` context |
| `--full` | Return the full document content per hit (use sparingly) |
| `-l` | Print matching paths only, deduplicated |
| `-c` | Print `path:count` per matching document |
| `-n` | Show line numbers (only meaningful with `--lines`) |
| `-C <n>` | Lines of context around each match (only meaningful with `--lines`) |

## Rules

- Queries are SQLite FTS5 syntax, not regular expressions. Bare words
  are ANDed; uppercase `AND`, `OR`, `NOT` and `NEAR()` are operators;
  `"exact phrase"` matches a sequence; `foo*` matches a prefix.
- A query with punctuation and no FTS5 operator is quoted for you and
  searched literally, so `foo-bar` and `"Authentication: OAuth2"` just
  work. Include an operator, or quote it yourself, for raw FTS5 syntax.
- Punctuation alone (`#`, `-`, `##`) matches nothing: the tokeniser
  strips it from the index. Anchor the search with a word next to it.
- A second argument restricts the search to documents under that path.
- Hits never carry the whole document unless you pass `--full`.
- At a terminal the output is rendered markdown; piped and over MCP it
  is raw markdown. `--json` gives a list of hits with `path`, `line`,
  `column`, `text`, `before`, `after` and `section`, whatever the mode.

## Examples

```bash
# Default: return matching markdown sections
llmd grep authentication

# Return line snippets with two lines of context
llmd grep --lines -C 2 authentication

# Line snippets with line numbers prefixed
llmd grep --lines -n authentication

# Whole documents per match (rare; opt-in)
llmd grep --full authentication

# Paths only - good for piping to xargs
llmd grep -l authentication | xargs -I{} llmd cat {}

# Match counts per document
llmd grep -c authentication

# Restrict to documents under a path prefix
llmd grep authentication api/

# Phrase search (exact sequence)
llmd grep '"hello world"'

# Prefix matching
llmd grep 'deploy*'

# Boolean OR
llmd grep 'postgres OR mysql'

# Proximity search (words within 5 tokens of each other)
llmd grep 'NEAR(deploy production)'

# Both of these are searched literally - no escaping needed
llmd grep foo-bar
llmd grep "Authentication: OAuth2"

# Structured JSON output for agents
llmd --json grep authentication
```

## Output format

Each match prints the document path on its own line followed by `:`,
then the match content underneath, with a blank line between matches:

```
notes/api:
## Authentication

OAuth2 with PKCE. Tokens expire after one hour.

guides/setup:
## Configuration

Set OAUTH_SECRET in your env file.
```

On an interactive terminal the output is rendered as markdown via
glamour. Pipes, redirects and MCP get raw markdown source. `--json`
gives the hits as a list with `path`, `line`, `column`, `text`,
`before`, `after` and `section`, whichever mode produced them.

## Search syntax

Queries use SQLite FTS5 syntax, **not** regular expressions:

- Bare words match implicit `AND` (all words must appear)
- Uppercase `AND`, `OR`, `NOT` are operators
- `NEAR(a b, 5)` finds `a` and `b` within 5 tokens of each other
- `foo*` matches words with that prefix
- `"exact phrase"` matches an exact word sequence

### Literal punctuation

Searches that contain punctuation but no FTS5 operators are
auto-quoted as literal phrases by the host bridge, so the common case
just works. The host detects that the query has no FTS5 operators
(`AND`, `OR`, `NOT`, `NEAR(`, `*`, or pre-existing `"`) and wraps it
as a phrase before sending it to FTS5. If you want raw FTS5 syntax,
include any of those operators or quote the query yourself.

### Limitation: pure punctuation

The default FTS5 tokeniser strips punctuation from the index entirely,
so searches for *only* punctuation - `grep #`, `grep -`, `grep ##` -
cannot match anything. The tokens aren't in the index. Use any
adjacent word to anchor the search: `grep "## Authentication"` works
because `Authentication` is a real token.

## Notes

llmd is an AI-first tool. The grep defaults are tuned so an agent
searching a long document gets back the matching markdown section
(bounded by headings), never the whole file. This keeps agent context
windows under control without any flags. Humans get the same data
glamour-rendered for the terminal; agents get raw markdown, or the
hits as JSON with `--json`.
