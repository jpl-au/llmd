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
- Implement the specification above
{{- if .Audits}}
- Address the audit feedback above
{{- end}}
- When done, move the task to review:
{{- if .URL}}
  `curl -sf -X POST "{{.URL}}/task/move/{{.Key}}?column=review" -H "Author: {{.Agent}}"`
{{- else}}
  `llmd --author {{.Agent}} task move {{.Key}} review`
{{- end}}
- The wrapper script will also handle this automatically on exit
