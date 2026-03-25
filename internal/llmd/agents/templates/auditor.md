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
- If approved: run `llmd --author {{.Agent}} task move {{.Key}} done`
- If issues found: run `llmd --author {{.Agent}} audit add {{.Key}} "<your feedback>"` then `llmd --author {{.Agent}} task move {{.Key}} in-progress`
