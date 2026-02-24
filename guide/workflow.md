# Workflow guide

Best practices for using llmd as a working document store.

## Set up your author

Every write records who made the change. Set your author name once:

```
llmd config author "Alice"
```

This writes to the local config at `.llmd/config`. To set it globally
(across all stores), use `--global`:

```
llmd config --global author "Alice"
```

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
