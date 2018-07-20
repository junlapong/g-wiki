{{ define "wiki" }}
  {{- if not .Content | and (not .Revision | or .IsHead) | or query.edit -}}
    {{- template "edit" . -}}
  {{- else -}}
    {{- template "view" . -}}
  {{- end -}}
{{ end }}
