{{ define "view" }}
{{- template "header" . -}}
{{- if .IsHead -}}
	{{- template "actions" . -}}
{{- else if .Revision -}}
	{{- template "revision" . -}}
{{- end -}}
{{- template "node" . -}}
{{- if query.show_revisions -}}
	{{- template "revisions" . -}}
{{- end -}}
{{- template "footer" . -}}
{{ end }}
