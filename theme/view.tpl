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
    <input type="text" class="form-control input-sm" name="path" placeholder="Path" value="{{ now.Format "2006-01-02-1504." }}" />
   </div>
  </form>
 </div>
{{- template "blog" . -}}
</div>
{{- end -}}
{{- template "footer" . -}}
{{ end }}
