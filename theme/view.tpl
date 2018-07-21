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
<p>Nodes</p>
<ul>
{{ range $f := glob "/*.md" }}
<li>{{ $f.Path }}</li>
{{ end }}
</ul>
{{- template "footer" . -}}
{{ end }}
