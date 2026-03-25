# Task: {{.Title}}

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

- Work in git branch `{{.Branch}}`
- Use `llmd` to read specs and update task status
- When done, run: `llmd --author {{.Agent}} task move {{.Key}} review`
- If you encounter issues, run: `llmd --author {{.Agent}} task move {{.Key}} failed`
