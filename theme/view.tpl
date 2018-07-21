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
<div class="row col-md-9">
 <div class="well">
  <!-- form class="form-inline" role="form"-->
  <form method="POST" class="navbar-form">
   <!-- div class="form-group"-->
   <div class="input-group">
    <div class="input-group-btn">
     <button type="submit" class="btn btn-default">New page</button>
    </div>
    <input type="text" class="form-control" id="new" placeholder="File name" />
   </div>
  </form>
 </div>
 <p>Nodes</p>
 <ul>
 {{ range $f := glob "/*.md" }}
 <li>{{ $f.Path }}</li>
 {{ end }}
 </ul>
</div>
{{- template "footer" . -}}
{{ end }}
