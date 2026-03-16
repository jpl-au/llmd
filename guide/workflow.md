# Workflow guide

Best practices for using llmd as a working document store.

## Set up your author

Every write records who made the change. Set your author name once:

```
llmd config author "Alice"
```

This writes to the local config at `.llmd/config.yaml`. To set it globally
(across all stores), use `--global`:

```
llmd config --global author "Alice"
```

AI agents must not use `config author` — pass `--author` on every
mutation command instead (see `guide llm`).

## Document path conventions

Paths use forward slashes and no leading slash. Use them like a
filesystem hierarchy:

```
projects/website/spec
projects/website/notes
meetings/2026-02-24
journal/today
```

Good conventions:
- Group related documents under a shared prefix
- Use dates for journals, meeting notes, and logs
- Keep paths short and descriptive

## Writing and iterating

Every `write` and `edit` creates a new version automatically. Use
`--message` to annotate why a change was made:

```
echo "First draft" | llmd write docs/proposal
echo "Revised draft" | llmd write docs/proposal --message "Added budget section"
llmd edit docs/proposal "TBD" "Q3 2026" --message "Confirmed timeline"
```

## Reviewing changes

Check what changed and when:

```
llmd history docs/proposal              # full version log
llmd history -n5 docs/proposal          # last 5 versions
llmd diff docs/proposal                 # diff against previous version
llmd diff docs/proposal:1 docs/proposal:3  # diff between specific versions
llmd cat --version 2 docs/proposal      # read an old version
```

## Using tags for workflow state

Tags are lightweight labels. Use them to track document state:

```
llmd tag docs/proposal draft
llmd tag docs/proposal review
llmd tag -d docs/proposal draft         # remove the draft tag
llmd tag -f review                      # find all docs tagged "review"
```

Common tag schemes:
- Status: `draft`, `review`, `final`, `archived`
- Priority: `urgent`, `next`, `backlog`
- Category: `spec`, `meeting`, `journal`

Tags work on any document, including task specs. Tag a task's spec
document to track additional state beyond the board column:

```
llmd tag tasks/fix-auth-tokens needs-design
```

List all tags and their counts:

```
llmd tag
```

## Linking related documents

Create directed links between documents:

```
llmd link meetings/2026-02-24 projects/website/spec
llmd link --label "blocked-by" tasks/auth tasks/db-migration
```

View links on a document:

```
llmd link projects/website/spec         # outgoing links
llmd link --in projects/website/spec    # incoming links
```

Link tasks to related documents with `task link`:

```
llmd task link a1b2c3d4e docs/api-spec
```

Linked documents appear in `review` output, giving reviewers context
without requiring them to search for related material.

## Search

Full-text search uses FTS5 syntax:

```
llmd grep budget                        # simple word search
llmd grep "budget AND timeline"         # boolean query
llmd grep budget projects/              # search within a prefix
llmd grep -l budget                     # paths only
llmd grep -n budget                     # with line numbers
```

Find documents by path pattern:

```
llmd glob "projects/*/spec"
llmd glob "meetings/2026-*"
```

## Listing and sorting

```
llmd ls                                 # all documents
llmd ls projects/                       # documents under a prefix
llmd ls -l                              # long format (version, author, date)
llmd ls -lt                             # long format, newest first
llmd ls -a                              # include deleted documents
```

## Deleting and recovering

Deletion is soft by default — documents are hidden but recoverable:

```
llmd rm docs/old-draft
llmd ls -a                              # shows deleted docs
llmd restore docs/old-draft             # bring it back
```

To permanently remove deleted documents and reclaim space:

```
llmd vacuum
```

## Reverting mistakes

Revert restores old content as a new version (history is never lost):

```
llmd revert docs/proposal 2             # revert to version 2
llmd revert docs/proposal v2            # "v" prefix also works
```

## Task lifecycle

Tasks track work through columns on a board:

  backlog → up-next → in-progress → review → done

These are the default columns. Customise them with `task column add`,
`task column rm`, and `task column mv`.

### Creating tasks

Every task has a backing spec document that describes the work:

```
llmd task add "Fix auth tokens" <<'SPEC'
## Context

Auth tokens never expire, causing security issues.

## Acceptance Criteria

- Tokens expire after 1 hour
- Expired tokens return 401
SPEC
```

Tasks cannot leave the backlog until the spec has content beyond the
title heading (spec gating). This prevents half-specified work from
moving forward.

### Moving tasks through the board

```
llmd task move a1b2c3d4e up-next        # ready to start
llmd task move a1b2c3d4e in-progress    # claimed, work underway
llmd task move a1b2c3d4e review         # work done, ready for review
llmd task finish a1b2c3d4e              # approved, move to done
```

Use `status` for a quick overview of the board and `review` to see
tasks with their spec previews:

```
llmd status                             # board counts + recent activity
llmd review                             # all tasks with context
llmd review --column review             # just what's waiting for review
```

### Reviewing tasks

When a task is in the review column, the reviewer should:

1. **Read the spec** — `task show <id>` to see what was asked for.
2. **Inspect the work** — check the code, document, or deliverable
   against the spec's acceptance criteria.
3. **Give feedback or approve:**
   - If changes are needed: `audit add <id> "description of issue"`
   - If the work is good: `task finish <id>`

Use `--assignee` on audit entries to direct feedback to the person
who did the work. They can check their inbox with `audit status`.

### Audit threads for review feedback

Audits are the feedback loop between contributors and reviewers.
They are immutable, insert-only threads attached to a task or document.

```
# Reviewer flags an issue
llmd --author "alice" audit add a1b2c3d4e "Error handling missing" \
  --assignee bob

# Coder checks their inbox and responds
llmd --author "bob" audit status
llmd --author "bob" audit reply <audit-id> "Fixed in latest commit"

# Reviewer approves the thread
llmd --author "alice" audit resolve <audit-id>

# Once all threads are resolved, finish the task
llmd --author "alice" task finish a1b2c3d4e
```

See `guide audit` for full details on threading, status values, and
filtering.

### Git integration

Link tasks to git branches for traceability:

```
git checkout -b feature-auth
llmd task start a1b2c3d4e               # records branch, moves to in-progress
llmd task diff a1b2c3d4e                # diff against default branch
llmd task files a1b2c3d4e               # list changed files
```

See `guide task` for full details on git integration, flags, and
metadata.

## Multi-agent collaboration

When multiple agents (or humans and agents) work together, llmd
coordinates through the task board and audit threads:

- **The board is the source of truth** for what needs doing, what's
  in progress, and what's waiting for review. Use `status` and `review`
  to stay oriented.
- **Audits are the communication channel.** Don't just move tasks
  around silently — leave audit trails explaining decisions, flagging
  issues, and confirming approvals.
- **`audit status` is your inbox.** Check it regularly to see threads
  assigned to you that need a response.
- **Author attribution matters.** Every mutation carries an author, so
  the history shows who did what. Agents must always pass `--author`.

A typical two-agent flow:

1. Agent A creates tasks from a spec and moves them to up-next.
2. Agent B picks up tasks, codes, and moves them to review.
3. Agent A reviews, creates audit threads for any issues.
4. Agent B checks `audit status`, fixes issues, replies to threads.
5. Agent A resolves audit threads and finishes the task.
