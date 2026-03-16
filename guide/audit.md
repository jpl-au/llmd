# llmd audit

Agent-to-agent and human-to-agent review threads. Audits are immutable,
insert-only records attached to documents or tasks. Thread status is
derived from the latest entry — no record is ever updated.

## Usage

```
llmd audit <subcommand> [options]
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `add <target> [content]` | Create a top-level audit |
| `reply <id> [content]` | Reply to an existing thread |
| `list [target]` | List audits (optionally filtered) |
| `show <id>` | Display full audit with thread |
| `resolve <id>` | Mark as approved |
| `rm <id>` | Soft-delete |
| `restore <id>` | Recover a soft-deleted audit |
| `status` | Inbox: what needs my response |

## Flags

| Flag | Description |
|------|-------------|
| `--status`, `-s` | Set or filter by status |
| `--assignee` | Assign to or filter by assignee |
| `--file` | Read content from a filesystem path |
| `--version` | Pin to a specific document version |
| `--pending` | Filter to pending/needs-work |
| `--by-author` | Filter by who created the audit |
| `--since` | Only show audits created after a time (e.g. `5m`, `1h`, RFC 3339) |

## Examples

### Create an audit

```bash
# Inline content
llmd --author "gemini" audit add docs/api "Error handling is incomplete."

# Content from stdin
echo "Detailed review..." | llmd --author "gemini" audit add docs/api

# Content from file
llmd --author "gemini" audit add docs/api --file review.md

# With status and assignee
llmd --author "gemini" audit add docs/api "Needs work." \
  --status needs-work --assignee claude-code
```

### Reply to an audit

```bash
# Simple reply
llmd --author "claude-code" audit reply 0mmsfn7h1 "Fixed."

# Reply and resolve in one action
llmd --author "claude-code" audit reply 0mmsfn7h1 --status approved

# Reassign to another agent
llmd --author "claude-code" audit reply 0mmsfn7h1 "Done, please check." \
  --assignee gemini
```

### Resolve an audit

```bash
# Quick resolve (inserts "approved" entry, no content needed)
llmd --author "claude-code" audit resolve 0mmsfn7h1
```

### List audits

```bash
# All audits
llmd audit list

# Audits on a specific document
llmd audit list docs/api

# Only pending/needs-work
llmd audit list --pending

# By creator
llmd audit list --by-author gemini

# By assignee
llmd audit list --assignee claude-code

# By exact status
llmd audit list --status needs-work

# Only audits from the last hour
llmd audit list --since 1h
```

### Show a full thread

```bash
llmd audit show 0mmsfn7h1
```

### Delete and restore

```bash
# Soft-delete
llmd --author "claude-code" audit rm 0mmsfn7h1

# Recover
llmd --author "claude-code" audit restore 0mmsfn7h1
```

### Check your inbox

```bash
# What needs my attention?
llmd --author "claude-code" audit status

# Uses config author in interactive terminals
llmd audit status
```

## How threading works

- All threads are single-level. Replying to a reply resolves to the
  top-level parent automatically.
- Thread status is the status of the most recent entry. A reply with
  `--status approved` both responds and resolves the thread.
- Assignee propagates through replies. If a reply omits `--assignee`,
  it inherits from the parent.

## How status works

`audit status` shows threads that need your attention:

- The thread is assigned to you (effective assignee), AND
- The effective status is `pending` or `needs-work`, AND
- The last message is not from you

Status values are free-form strings. Common conventions:
`pending`, `approved`, `needs-work`, `rejected`, `info`.

## Target type inference

The store determines whether the target is a document or task
automatically. Valid 9-character base36 keys are treated as task IDs;
everything else is a document path.

## Notes

- Author is required for mutations (`add`, `reply`, `resolve`, `rm`,
  `restore`). Reads (`list`, `show`) work without an author.
- Content resolution order: positional argument > `--file` > stdin.
- IDs are bare 9-character base36 keys with no prefix.
- Use `--json` for structured output.
