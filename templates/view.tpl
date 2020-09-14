{{ define "view" }}
{{- template "header" . -}}
{{- if .IsHead | or (not .Revision) -}}
  {{- template "actions" . -}}
{{- else -}}
  {{- template "revision" . -}}
{{- end -}}
{{- template "node" . -}}
{{- if query.show_revisions -}}
  {{- template "revisions" . -}}
{{- end -}}
{{- if eq .Path "/" }}
  {{- template "blog" . -}}
{{- else if .Path | matchre `^/20\d\d-\d\d/?$` -}}
  {{- template "blog" . -}}
{{- else if .Path | matchre `^/20\d\d-\d\d-\d\d/?$` -}}
  {{- template "blog" . -}}
{{- else if .Path | matchre `^/tag/` -}}
<div class="row col-md-9">
  {{- $tag := .Path | matchre `^/tag/(.*)` -}}
  {{- $glob := printf "/20??-??-??*.@%s.*" $tag -}}
  {{- template "blog-list" $glob -}}
</div>
{{- end -}}
{{- template "footer" . -}}
{{ end }}
