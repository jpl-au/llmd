# Workflow Guide

How to use llmd effectively as an autonomous workspace.

## Author Attribution

**Always use `-a` to identify yourself on every write operation:**

```bash
echo "content" | llmd write docs/notes -a "claude-code"
llmd edit docs/notes "old" "new" -a "claude-code"
llmd sed -i 's/foo/bar/' docs/notes -a "claude-code"
```

This creates an audit trail showing who changed what. Without `-a`, changes cannot be attributed to you.

## Why llmd

llmd provides a single document store that keeps your work organised without littering files across the filesystem. All documents, versions, and history live in one database. This makes it ideal for:

- Iterative work that evolves over multiple sessions
- Collaboration between humans and LLMs
- Maintaining context and history across long-running projects

## Best Practices

### Clear Completion Criteria

Define what "done" looks like upfront. Vague goals lead to aimless iteration.

```markdown
# Task: API Documentation

## Done When
- All endpoints documented
- Request/response examples for each
- Error codes listed
```

Without clear criteria, you cannot know when to stop or whether you've succeeded.

### Incremental Goals

Break large objectives into smaller, verifiable steps. Complete each before moving to the next.

```markdown
## Phases
1. Document authentication endpoints
2. Document user endpoints
3. Document admin endpoints
```

Each phase should be independently completable. This allows progress even when the full task is complex.

### Self-Correction

Build in checkpoints to review and correct your work. Use llmd's versioning:

```bash
llmd diff docs/api           # what changed?
llmd history docs/api        # how did we get here?
llmd cat docs/api -v 2       # was version 2 better?
```

When something isn't working, review previous versions to understand what went wrong.

### Know When to Stop

Define escape conditions. Not every problem is solvable in one session.

```markdown
## If Blocked
- Document what was attempted
- Describe the blocker
- Set status to "needs-review"
```

Escalating to a human with good context is better than spinning indefinitely.

## Philosophy

**Iteration over perfection** - First drafts are rarely right. Write, review, refine.

**Failures are data** - A failed approach tells you something. Document it and try differently.

**History is context** - Version history shows how you got here. Use it to inform next steps.

**One source of truth** - Everything in llmd. No scattered files, no lost context.

## Tools Reference

### Reading

| Command | Purpose |
|---------|---------|
| `llmd cat <path\|key>` | Read a document |
| `llmd cat <path\|key> -v N` | Read specific version |
| `llmd cat <path\|key> -o json` | Read with metadata |
| `llmd ls` | List all documents |
| `llmd ls <prefix>` | List under a path |
| `llmd ls -t` | Tree view |
| `llmd glob "pattern"` | Match paths by pattern |
| `llmd find "term"` | Full-text search |

### Writing

| Command | Purpose |
|---------|---------|
| `echo "..." \| llmd write <path>` | Create/overwrite document |
| `llmd edit <path\|key> "old" "new"` | Search and replace |
| `llmd edit <path\|key> -l 5:10 < new.txt` | Replace line range |

Always use `-a` to identify yourself:

```bash
echo "content" | llmd write docs/notes -a claude
llmd edit docs/notes "draft" "final" -a claude -m "Finalised"
```

### History

| Command | Purpose |
|---------|---------|
| `llmd history <path\|key>` | Show all versions |
| `llmd diff <path\|key>` | Compare latest vs previous |
| `llmd diff <path\|key> -v 1:3` | Compare specific versions |

### Organisation

| Command | Purpose |
|---------|---------|
| `llmd mv <old> <new>` | Move/rename |
| `llmd rm <path\|key>` | Soft delete |
| `llmd restore <path\|key>` | Restore deleted |

### Tagging

| Command | Purpose |
|---------|---------|
| `llmd tag add <path\|key> <tag>` | Add tag |
| `llmd tag rm <path\|key> <tag>` | Remove tag |
| `llmd tag ls [path\|key]` | List tags |
| `llmd ls --tag <tag>` | Filter by tag |

## Example: Working Session

```bash
# Start by reading current state
llmd cat project/status

# Check what exists
llmd ls project/

# Do work, write results
echo "## Analysis\n\nFindings here..." | llmd write project/analysis -a claude

# Tag as draft
llmd tag add project/analysis "draft"

# Review what you wrote
llmd cat project/analysis

# Refine it
llmd edit project/analysis "Findings here" "Key finding: X affects Y" -a claude

# Mark as ready for review
llmd tag rm project/analysis "draft"
llmd tag add project/analysis "needs-review"

# Check the diff
llmd diff project/analysis

# Update status
llmd edit project/status "Phase: research" "Phase: complete" -a claude
```

## Document Conventions

Paths use forward slashes, no leading slash. The `.md` extension is automatically stripped:

```
docs/api/auth       (stored path)
docs/api/auth.md    (automatically becomes docs/api/auth)
/docs/api/auth      (wrong - no leading slash)
```

Use `-o json` when you need structured output for parsing.
