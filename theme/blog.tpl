{{- define "blog" -}}
<ul>
{{ range $a := glob "/20??-??-??*.md" | reverse }}
 {{- $oneline := $a.Content | matchre `^\s*([^\n]+)\s*$` -}}
 {{ if $oneline }}
  <li>{{ printf "%s&emsp;[¶](%s)" $oneline $a.Path | markdown }}</li>
 {{ else }}
</ul>
{{/* TODO(akavel): embed, but not in a well (hline? dashed? asterism? fleurons?); show permalink appropriate for multiline content */}}
<div class="well">{{ $a.Content | markdown }}</div>
<ul>
 {{ end }}
{{ end }}
</ul>
{{- end -}}

