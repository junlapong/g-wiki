{{- define "blog-list" -}}
 <ul>
 {{- $prev := "" -}}
 {{- range $a := glob . | reverse -}}
  {{- $oneline := $a.Content | matchre `^\s*([^\n]+)\s*$` -}}
  {{- if $oneline -}}
   {{- if $prev | eq "multiline" }}
 <p style="text-align:center">─── &emsp;&emsp;❖&emsp;&emsp; ───</p>
 <ul>
   {{- end -}}
   {{- $html := $oneline | markdown | matchre `^\s*<p>(.*)</p>\s*$` }}
  <li><p>{{ $html }}&emsp;<a href="{{ $a.Path }}">¶</a> {{ template "tag" $a.Path }}</p></li>
   {{- $prev = "oneline" -}}
  {{- else -}}
   {{- if $prev | eq "multiline" | not }}
 </ul>
   {{- end }}
 <p style="text-align:center">───── &emsp; <a style="font-weight: bold; font-size: larger" href="{{ $a.Path }}">&emsp;§&emsp;</a> &emsp; ─────</p>
 <div style="float:right">{{ template "tag" $a.Path }}</div>
{{ $a.Content | markdown -}}
   {{- $prev = "multiline" -}}
  {{- end -}}
 {{- end }}
 </ul>
{{- end -}}
