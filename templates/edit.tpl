{{ define "edit" }}
{{ template "header" . }}
<div class="row col-md-9">
<form method="POST" enctype="multipart/form-data">
 <div class="form-group">
  <div class="input-group">
   <span class="input-group-addon">/</span>
   <!-- glyphicon-link -->
   <input type="text" class="form-control input-sm" name="rename" value="{{ .File }}" />
  </div>
 </div>
 <div class="form-group col-md-12">
  <textarea type="text" class="form-control" rows="15" placeholder="Insert markdown here" name="content">{{ .Content }}</textarea>
 </div>
 <div class="form-group">
  <div class="input-group">
   <input type="file" class="form-control input-sm" name="attachment" />
   <div class="input-group-btn">
    <button type="submit" class="btn btn-default btn-sm" name="edit" value="true">
     <!-- span class="glyphicon glyphicon-floppy-open"></span -->
     <span class="glyphicon glyphicon-upload"></span> Attach
    </button>
   </div>
  </div>
 </div>
 <div class="form-inline col-md-12">
  <div class="form-group col-md-8">
   <input type="text" class="form-control" name="msg" placeholder="Changelog"/>
  </div>
  <div class="form-group col-md-2">
   <input type="text" class="form-control" name="author" placeholder="Author"/>
  </div>
  <div class="form-group col-md-2">
   <button type="submit" class="btn btn-default">
    <span class="glyphicon glyphicon-floppy-disk"></span> Save
   </button>
  </div>
 </div>
</form>
</div>
{{ template "footer" . }}
{{ end }}
