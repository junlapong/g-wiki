{{define "revisions"}}
<div class="row col-md-9">
 <hr />
 <p class="text-muted">Revisions:</p>
 <div class="list-group">
  {{$node := .}}
  {{range $revision := .Revisions}}
   {{if eq $revision.Hash $node.Revision}}
    <a href="?revision={{$revision.Hash}}&show_revisions=1" class="list-group-item active">
   {{else}}
    <a href="?revision={{$revision.Hash}}&show_revisions=1" class="list-group-item">
   {{end}}
    {{$revision.Message}} ({{$revision.Time}})
   </a>
   </li>
  {{end}}
 </div>
</div>
{{end}}
