# Review: {{.Title}}

- **Task:** {{.Key}}
- **Branch:** {{.Branch}}

## Specification

{{.Spec}}
{{if .Diff}}
## Changes to Review

```
{{.Diff}}
```
{{end}}
{{- if .Audits}}
## Previous Audit History
{{range .Audits}}
**{{.Author}}** ({{.Status}}): {{.Content}}
{{end}}
{{- end}}
## Instructions

- Review the changes against the specification
- If approved, move the task to done:
{{- if .URL}}
  `curl -sf -X POST "{{.URL}}/task/move/{{.Key}}?column=done" -H "Author: {{.Agent}}"`
{{- else}}
  `llmd --author {{.Agent}} task move {{.Key}} done`
{{- end}}
- If issues found, add an audit reply then move back to in-progress:
{{- if .URL}}
  `curl -sf -X POST "{{.URL}}/audit/add/{{.Key}}" -H "Author: {{.Agent}}" -d "<your feedback>"`
  `curl -sf -X POST "{{.URL}}/task/move/{{.Key}}?column=in-progress" -H "Author: {{.Agent}}"`
{{- else}}
  `llmd --author {{.Agent}} audit add {{.Key}} "<your feedback>"`
  `llmd --author {{.Agent}} task move {{.Key}} in-progress`
{{- end}}
- The wrapper script will handle task moves automatically on exit
