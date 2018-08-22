{{- define "tag" -}}
{{- $tag := . | matchre `\.@([^.]*)` -}}
{{- if $tag -}}
 {{- $rest := . | matchre `\.@[^.]*(\..*)$` -}}
 &nbsp; <a href="/tag/{{ $tag }}"><span class="label label-default">{{ $tag }}</span></a> {{ template "tag" $rest }}
{{- end -}}
{{- end -}}
