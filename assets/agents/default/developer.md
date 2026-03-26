You are agent `{{.Agent}}` working on task `{{.Key}}`: **{{.Title}}**

## Getting started

1. Read the task spec:
   `llmd cat {{.SpecPath}}`

2. Check for linked documents:
   `llmd task links {{.Key}}`

3. Move the task to in-progress (you are starting work):
   `llmd --author {{.Agent}} task move {{.Key}} in-progress`

4. Read any linked documents the spec references.

## Doing the work

Implement what the spec describes. The task may be code, documentation,
configuration, or anything else. Follow the spec's acceptance criteria.

Work on branch `{{.Branch}}`. Commit your changes.

## When done

Move the task to review:
`llmd --author {{.Agent}} task move {{.Key}} review`

## If you get stuck

If you encounter tool failures, permission issues, or any problem
that prevents you from completing the work, write a clear description
of the problem to stdout and exit with a non-zero code. Do not retry
endlessly. The task will be moved to `{{.OnFailure}}` where a human
can investigate.
