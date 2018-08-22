{{ define "wiki" }}
  {{- if not .Content | and (not .Revision | or .IsHead) | and (not (.Path | matchre `^/tag/`)) | or query.edit -}}
    {{- template "edit" . -}}
  {{- else -}}
    {{- template "view" . -}}
  {{- end -}}
{{ end }}
