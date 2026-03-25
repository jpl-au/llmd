package agents

// Default prompt templates seeded when an agent is registered.
// These use Go text/template syntax with the context data passed
// during spawning.

// DefaultDeveloperPrompt is the built-in developer prompt template.
const DefaultDeveloperPrompt = `# Task: {{.Title}}

- **ID:** {{.Key}}
- **Branch:** {{.Branch}}
{{- if .AssignedTo}}
- **Assigned to:** {{.AssignedTo}}
{{- end}}

## Specification

{{.Spec}}
{{if .LinkedDocs}}
## Linked Documents
{{range .LinkedDocs}}
### {{.Path}}

{{.Content}}
{{end}}
{{- end}}
{{- if .Audits}}
## Audit History
{{range .Audits}}
**{{.Author}}** ({{.Status}}): {{.Content}}
{{end}}
{{- end}}
## Instructions

- Work in git branch ` + "`{{.Branch}}`" + `
- Use ` + "`llmd`" + ` to read specs and update task status
- When done, run: ` + "`llmd --author {{.Agent}} task move {{.Key}} review`" + `
- If you encounter issues, run: ` + "`llmd --author {{.Agent}} task move {{.Key}} failed`" + `
`

// DefaultAuditorPrompt is the built-in auditor prompt template.
const DefaultAuditorPrompt = `# Review: {{.Title}}

- **Task:** {{.Key}}
- **Branch:** {{.Branch}}

## Specification

{{.Spec}}
{{if .Diff}}
## Changes to Review

` + "```" + `
{{.Diff}}
` + "```" + `
{{end}}
{{- if .Audits}}
## Previous Audit History
{{range .Audits}}
**{{.Author}}** ({{.Status}}): {{.Content}}
{{end}}
{{- end}}
## Instructions

- Review the changes against the specification
- If approved: run ` + "`llmd --author {{.Agent}} task move {{.Key}} done`" + `
- If issues found: run ` + "`llmd --author {{.Agent}} audit add {{.Key}} \"<your feedback>\"`" + ` then ` + "`llmd --author {{.Agent}} task move {{.Key}} in-progress`" + `
`
