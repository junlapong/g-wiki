{{ define "view" }}
{{- template "header" . -}}
{{- if .IsHead -}}
	{{- template "actions" . -}}
{{- else if .Revision -}}
	{{- template "revision" . -}}
{{- end -}}
{{- template "node" . -}}
{{- if query.show_revisions -}}
	{{- template "revisions" . -}}
{{- end -}}
{{- if eq .Path "/" }}
<div class="row col-md-9">
 <div class="well">
  <!-- form class="form-inline" role="form"-->
  <form method="POST" class="navbar-form">
   <!-- div class="form-group"-->
   <div class="input-group">
    <div class="input-group-btn">
     <button type="submit" class="btn btn-default" name="edit" value="true">Edit page</button>
    </div>
    <input type="text" class="form-control" name="path" placeholder="Path" />
   </div>
  </form>
 </div>

 <ul>
{{ range $f := glob "/20??-??-??*.md" | reverse }}
 {{ if $f.Path | matchre "[0-9]quick\\." }}
  <li>{{ $f.Markdown }}</li>
 {{ else }}
 </ul>
 <div class="well">{{ $f.Markdown }}</div>
 <ul>
 {{ end }}
{{ end }}
 </ul>
</div>
{{- end -}}
{{- template "footer" . -}}
{{ end }}
