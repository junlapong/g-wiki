{{ define "wiki" }}
  {{- if not .Content | and (not .Revision | or .IsHead) | and (not (.Path | matchre `^/tag/`)) | and (not (.Path | matchre `^/20\d\d[-\d]*/?$`)) | or query.edit -}}
    {{- template "edit" . -}}
  {{- else -}}
    {{- template "view" . -}}
  {{- end -}}
{{ end }}
