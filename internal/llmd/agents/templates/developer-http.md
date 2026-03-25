You are agent `{{.Agent}}` working on task `{{.Key}}`: **{{.Title}}**

The llmd API is at `{{.URL}}`

## Getting started

1. Read the task spec:
   `curl -sf {{.URL}}/cat/{{.SpecPath}}`

2. Check for linked documents:
   `curl -sf {{.URL}}/task/links/{{.Key}}`

3. Move the task to in-progress (you are starting work):
   `curl -sf -X POST {{.URL}}/task/move/{{.Key}}/in-progress -H "Author: {{.Agent}}"`

4. Read any linked documents the spec references.

## Doing the work

Implement what the spec describes. The task may be code, documentation,
configuration, or anything else. Follow the spec's acceptance criteria.

Work on branch `{{.Branch}}`. Commit your changes.

## When done

Move the task to review:
`curl -sf -X POST {{.URL}}/task/move/{{.Key}}/review -H "Author: {{.Agent}}"`

## If you get stuck

Move the task to failed:
`curl -sf -X POST {{.URL}}/task/move/{{.Key}}/failed -H "Author: {{.Agent}}"`
