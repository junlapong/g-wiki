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
</div>
{{- end -}}

