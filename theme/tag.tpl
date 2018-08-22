{{- define "tag" -}}
{{- $tag := . | matchre `\.@([^.]*)` -}}
{{- if $tag -}}
 {{- $rest := . | matchre `\.@[^.]*(\..*)$` -}}
 &nbsp; <span class="label label-default">{{ $tag }}</span> {{ template "tag" $rest }}
{{- end -}}
{{- end -}}
