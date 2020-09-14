package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/russross/blackfriday.v1"
)

type node struct {
	Path      string
	File      string
	Content   string
	Revision  string
	Dirs      []*directory
	Revisions []*revision

	// FIXME(akavel): this should not have to be here
	repo repository
}

type directory struct {
	Path string
	Name string
}

type revision struct {
	Hash    string
	Message string
	Time    string
}

func (node *node) IsHead() bool {
	return len(node.Revisions) > 0 && node.Revision == node.Revisions[0].Hash
}

func parseLog(line string) *revision {
	// TODO(akavel): allow showing page diffs, maybe as method on revision type
	re := regexp.MustCompile(`(.{0,7}) (\d+ \w+ ago) (.*)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 4 {
		return &revision{Hash: matches[1], Time: matches[2], Message: matches[3]}
	}
	return nil
}

func listDirectories(path string) []*directory {
	var s []*directory
	dirPath := ""
	for i, dir := range strings.Split(path, "/") {
		if i == 0 {
			dirPath += dir
		} else {
			dirPath += "/" + dir
		}
		s = append(s, &directory{Path: dirPath, Name: dir})
	}
	return s
}

// Soft reset to specific revision
func (node *node) gitRevert() *node {
	log.Printf("Reverts %s to revision %s", node.File, node.Revision)
	node.repo.git("checkout", node.Revision, "--", node.File)
	return node
}

func writeFile(entry string, bytes []byte) error {
	// FIXME(akavel): make sure to sanitize the 'entry' path
	err := os.MkdirAll(path.Dir(entry), 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(entry, bytes, 0644)
}

func (wiki *wikiHandler) renderTemplate(w http.ResponseWriter, node *node, query map[string]string) {
	funcs := template.FuncMap{
		"query":   func() map[string]string { return query },
		"inc":     func(i int) int { return i + 1 },
		"glob":    wiki.glob,
		"matchre": matchre,
		"reverse": reverse,
		// TODO(akavel): allow specifying options, for safety
		"markdown": markdown,
		"now":      time.Now,
		"path":     func() string { return node.Path },
	}

	t, err := template.New("wiki").Funcs(funcs).ParseGlob(wiki.TemplateGlob)
	if err != nil {
		log.Print("Could not parse template:", err)
		// TODO(akavel): at least print a fallback simple HTML of the node for viewing
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, node)
	if err != nil {
		log.Printf("Could not execute template for node %q: %s", node.Path, err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
	}
}

// NOTE: glob pattern must not start with "/" here
func (wiki *wikiHandler) glob(glob string) []*node {
	rawGlob, glob := glob, cleanPath(glob)
	// TODO(akavel): use -z option, or unquote filenames to be safe with special chars
	lines := string(wiki.Repo.git("ls-tree", "HEAD", "--", path.Dir(glob)+"/"))
	// Example lines emitted by `git ls-tree` (blob means a file, tree means a directory):
	//
	// 040000 tree ef240d2545ebf7e8a04ff09b9a0b5686782c06e4	theme
	// 100755 blob bb3b016d78458c9b8ef1549597e77f44529905fc	wiki.go
	re := regexp.MustCompile(`(\S+) (\S+) (\S+)\t(.*)`)
	var nodes []*node
	var wg sync.WaitGroup

	for _, line := range strings.Split(lines, "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		typ, file := m[2], m[4]
		if typ != "blob" || !strings.HasSuffix(file, ".md") {
			continue
		}

		match, err := path.Match(glob, file)
		if err != nil {
			// According to path.Match godoc: "The only possible
			// returned error is ErrBadPattern, when pattern is
			// malformed"
			log.Printf("glob: %v: %q", err, rawGlob)
			return nil
		}

		if !match {
			continue
		}

		node := &node{
			Path: "/" + strings.TrimSuffix(file, ".md"),
			File: file,
			Dirs: listDirectories("/" + file),
			repo: wiki.Repo,
		}
		nodes = append(nodes, node)
		wg.Add(1)
		go func() {
			defer wg.Done()
			node.gitShow()
		}()
	}

	wg.Wait()
	return nodes
}

func markdown(md string) template.HTML {
	return template.HTML(blackfriday.MarkdownCommon([]byte(md)))
}

func matchre(pattern string, s interface{}) (interface{}, error) {
	var text string
	html := false

	switch s := s.(type) {
	case string:
		text = s
	case template.HTML:
		text = string(s)
		html = true
	default:
		return nil, fmt.Errorf("matchre: unexpected type of argument: %T", s)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}

	m := re.FindStringSubmatch(text)
	switch len(m) {
	case 0:
		text = ""
	case 1:
		text = m[0]
	default:
		text = m[1]
	}

	if html {
		return template.HTML(text), nil
	}
	return text, nil
}

// TODO(akavel): make it work not only on nodes, but any slice & string (with reflect pkg)
func reverse(nodes []*node) []*node {
	n := len(nodes)
	r := make([]*node, n)

	for i := range r {
		r[i] = nodes[n-i-1]
	}
	return r
}

// TODO(akavel): use this in all places where Clean/TrimLeft/ToSlash is used
// TODO(akavel): somehow check if this is enough sanitization or not yet (vs. filepath.Clean?)
func cleanPath(p string) string {
	return strings.TrimLeft(path.Clean(filepath.ToSlash("/"+p)), "/")
}

// HTTPRejectGlob returns a 404 Not Found error in case the request path (normalized) matches the glob.
// Otherwise (when glob doesn't match), h is called.
func HTTPRejectGlob(glob string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(strings.TrimLeft(r.URL.Path, "/"))
		match, err := filepath.Match(glob, path)
		if err != nil {
			log.Println(err)
			match = true
		}
		if match {
			http.NotFound(w, r)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

type wikiHandler struct {
	Repo         repository
	TemplateGlob string
}

func (wiki *wikiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	log.Printf("%s\t%s\n", r.Method, r.RequestURI)

	urlPath := cleanPath(r.URL.Path)
	if p := cleanPath(r.FormValue("path")); p != "" && p != urlPath {
		RedirectWithData(w, r, strings.TrimSuffix(p, ".md"))
		return
	}
	// Don't show any files or directories with names starting with "." (especially ".git")
	for _, segment := range strings.Split(urlPath, "/") {
		if strings.HasPrefix(segment, ".") {
			http.NotFound(w, r)
			return
		}
	}

	// "Clean URLs" - strip .md suffix from URL path
	// TODO(akavel): make below work also on case-insensitive filesystems
	if strings.HasSuffix(urlPath, ".md") {
		RedirectWithData(w, r, strings.TrimSuffix(urlPath, ".md"))
		return
	}

	// If a requested non-.md file exists on disk, return it, under assumption that it is a static resource
	if serveFile(w, r, filepath.Join(string(wiki.Repo), urlPath)) {
		return
	}

	switch urlPath {
	case "favicon.ico":
		http.NotFound(w, r)
		return
	case "index.html":
		urlPath = ""
	}

	// Params
	var (
		content   = r.FormValue("content")
		changelog = r.FormValue("msg")
		author    = r.FormValue("author")
		reset     = r.FormValue("revert")
		revision  = r.FormValue("revision")
		rename    = cleanPath(r.FormValue("rename"))
	)

	query := map[string]string{
		"edit":           r.FormValue("edit"),
		"show_revisions": r.FormValue("show_revisions"),
	}

	node := &node{
		Path: "/" + urlPath,
		File: urlPath + ".md",
		Dirs: listDirectories(urlPath),
		repo: wiki.Repo,
	}

	// TODO(akavel): maybe save to subdir named after node's filename
	// TODO(akavel): or, create a random filename (or suffix) when saving
	// TODO(akavel): [LATER-safety] add flag for restricting allowed file extensions

	if r.Method == "POST" {
		attachment, err := handleUpload(r, "attachment", filepath.Join(string(wiki.Repo), path.Dir(node.File)))
		if err != nil {
			log.Println(err)
		} else if attachment != "" {
			node.repo.git("add", "--", filepath.Join(path.Dir(node.File), attachment))
			node.repo.git("commit", "-m", "Uploaded: "+attachment)
			content += fmt.Sprintf("\n[%[1]s](%[1]s)", attachment)
		}
	}

	switch {
	case rename != "" && rename != urlPath && rename != node.File:
		if !strings.HasSuffix(rename, ".md") {
			rename += ".md"
		}
		// TODO(akavel): print some message to user in case of failure
		node.repo.git("mv", node.File, rename)
		node.gitCommit("Rename "+node.File+" to "+rename, author)
		RedirectWithData(w, r, rename)
		return
	case content != "":
		if changelog == "" {
			changelog = "Update " + node.File
		}
		filePath := filepath.Join(string(wiki.Repo), node.File)
		bytes := normalize([]byte(content))
		if *verbose {
			log.Printf("(writing %d bytes to file %q)", len(bytes), filePath)
		}
		err := writeFile(filePath, bytes)
		if err != nil {
			log.Printf("Can't write to file %q, error: %v", filePath, err)
		} else {
			// Wrote file, commit
			node.Content = string(bytes)
			node.gitAdd().gitCommit(changelog, author).gitLog()
		}
		// TODO(akavel): redirect to normal page, to shake off POST on browser refresh
	case reset != "":
		// Reset to revision
		if *verbose {
			log.Printf("(resetting %q to revision %s)", node.File, reset)
		}
		node.Revision = reset
		node.gitRevert().gitCommit("Reverted to: "+node.Revision, author)
		node.Revision = ""
		node.gitShow().gitLog()
		// TODO(akavel): redirect to normal page, to shake off POST on browser refresh
	default:
		// Show specific revision
		if *verbose {
			log.Printf("(showing %q at revision %s)", node.File, revision)
		}
		node.Revision = revision
		node.gitShow().gitLog()
	}
	wiki.renderTemplate(w, node, query)
}

func serveFile(w http.ResponseWriter, r *http.Request, path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	if stat.IsDir() {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		log.Println("Cannot serveFile:", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return true
	}
	defer f.Close()
	http.ServeContent(w, r, path, stat.ModTime(), f)
	return true
}

func normalize(buf []byte) []byte {
	// convert Windows CR-LFs to Unix LFs
	buf = bytes.Replace(buf, []byte("\r\n"), []byte("\n"), -1)
	// make sure there are no remaining CRs
	buf = bytes.Replace(buf, []byte("\r"), []byte("\n"), -1)
	return buf
}

func handleUpload(r *http.Request, field, dir string) (string, error) {
	upload, info, err := r.FormFile("attachment")
	if err == http.ErrMissingFile {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("opening %s: %v", field, err)
	}
	defer upload.Close()
	f, err := os.OpenFile(filepath.Join(dir, info.Filename), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return "", fmt.Errorf("saving %s: %v", field, err)
	}
	defer f.Close()
	_, err = io.Copy(f, upload)
	if err != nil {
		return "", fmt.Errorf("writing %s: %v", field, err)
	}
	err = f.Close()
	if err != nil {
		return "", fmt.Errorf("closing %s: %v", field, err)
	}
	return info.Filename, nil
}

// RedirectWithData writes a 307 redirect, which orders the browser to re-send POST data.
// The function also keeps the URL query parameters.
func RedirectWithData(w http.ResponseWriter, r *http.Request, newPath string) {
	r.URL.Path = "/" + strings.TrimLeft(newPath, "/")
	http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
}
