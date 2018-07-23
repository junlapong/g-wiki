{{- define "blog" -}}
<div class="row col-md-9">
 <div class="navbar navbar-default">
  <!-- form class="form-inline" role="form"-->
  <form method="POST" class="navbar-form">
   <!-- div class="form-group"-->
   <div class="input-group">
    <div class="input-group-btn">
     <button type="submit" class="btn btn-default btn-sm" name="edit" value="true">
      <span class="glyphicon glyphicon-file"></span> Edit page
     </button>
    </div>
    <input type="text" class="form-control input-sm" name="path" placeholder="Path" value="{{ now.Format "2006-01-02-150405." }}" />
   </div>
  </form>
 </div>
 <ul>
 {{ $prev := "" }}
 {{ range $a := glob "/20??-??-??*.md" | reverse }}
  {{- $oneline := $a.Content | matchre `^\s*([^\n]+)\s*$` -}}
  {{ if $oneline }}
   {{ if $prev | eq "multiline" }}
  <p style="text-align:center">─── &emsp;&emsp;❖&emsp;&emsp; ───</p>
  <ul>
   {{ end }}
   <li>{{ printf "%s&emsp;[¶](%s)" $oneline $a.Path | markdown }}</li>
   {{ $prev = "oneline" }}
  {{ else }}
   {{ if $prev | eq "multiline" | not }}
 </ul>
   {{ end }}
  <p style="text-align:center">─── &emsp; <a style="font-weight: bold" href="{{ $a.Path }}">&emsp;§&emsp;</a> &emsp; ───</p>
  <!-- p style="text-align:center">☙&emsp;❖&emsp;❧</p -->
  <!-- p style="text-align:center">☙&emsp;⧫&emsp;❧</p -->
  <!-- p style="text-align:center">☙&emsp;✽&emsp;❧</p -->
  <!-- p style="text-align:center">☙&emsp;✵&emsp;❧</p -->
  <!-- p style="text-align:center">🙡&emsp;❖&emsp;🙣</p -->
  <!-- p style="text-align:center">🙜&emsp;✽&emsp;🙞</p -->
  <!-- p style="text-align:center">🙪&emsp;🙪&emsp;🙪</p -->
  <!-- p style="text-align:center">☙&emsp;🏶&emsp;❧</p -->
  <!-- p style="text-align:center">⬥&emsp;❖&emsp;⬥</p -->
{{ $a.Content | markdown }}
   {{ $prev = "multiline" }}
  {{ end }}
 {{ end }}
 </ul>
</div>
{{- end -}}

