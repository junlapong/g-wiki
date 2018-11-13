{{- define "blog-list" -}}
 <ul>
 {{- $prefixshow := path | matchre `^/20\d\d[-\d]*` -}}
 {{- $prefixlen := len $prefixshow -}}
 {{- $prefixre := printf `^.{%d}` $prefixlen -}}
 {{- $prefixprev := "" -}}
 {{- $prev := "" -}}

 {{- range $a := glob . | reverse -}}

  {{- $prefix := $a.Path | matchre $prefixre -}}
  {{- if eq $prefix $prefixshow -}}
   {{- if ne $prefixprev $prefix -}}
 </ul>
 <p><a href="{{ $prefixprev }}">&lt;&lt; newer...</a></p>
 <ul>
   {{- end -}}

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

  {{- else if lt $prefix $prefixshow -}}
   {{- if eq $prefixprev $prefixshow -}}
    {{- if $prev | eq "multiline" | not }}
 </ul>
    {{- end }}
 <p><a href="{{ $prefix }}">older... &gt;&gt;</a></p>
   {{- end -}}
  {{- end -}}
  {{- $prev = $prefix -}}
 {{- end }}
 </ul>
{{- end -}}
