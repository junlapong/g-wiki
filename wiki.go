// g-wiki - a simple wiki built with Go, storing Markdown files in Git as its back-end.
// Copyright (C) 2014-2018  The g-wiki Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/russross/blackfriday"
)

// TODO(akavel): fix FIXMEs (sanitization of paths, etc.)
// TODO(akavel): allow deleting files from repo
// TODO(akavel): allow adding file attachments into the wiki (images, etc. - probably restrict extensions via flag)
// TODO(akavel): [LATER] nice JS editor, with preview of markdown... but how to ensure compat. with blackfriday? or, VFMD everywhere?.........
// TODO(akavel): [MAYBE] use pure Go git implementation, maybe go-git; but this may increase complexity too much

func usage() {
	fmt.Fprintf(os.Stderr, `USAGE: %s [FLAGS]

Starts a simple wiki service using git as the storage back-end. Content is
formatted in markdown syntax.

WARNING: the wiki has no protections against malicious editing, and no support
for multiple simultaneous editors.

FLAGS:
`, os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
THEMING:

Theme files must define a 'wiki' template (see https://golang.org/pkg/html/template).
The following object is available in the template:

	struct {
		Path     string
		File     string
		Content  string
		Revision string
		Dirs []*struct{
			Path string
			Name string
		}
		Revisions []*struct {
			Hash    string
			Message string
			Time    string
		}

		IsHead   bool
	}

Additionally, the following functions are available in the template:

	markdown STRING
	        - returns HTML render of a Markdown-formatted argument
	          Warning: not safe on user-submitted content such as comments
	query   - returns a map providing access to the following URL parameters:
	        query.edit
	        query.show_revisions
	inc INT - returns the INT value incremented by +1
	glob PATTERN
	        - returns a list of pages matching the file pattern
	reverse - returns a list of pages with swapped order
	matchre PATTERN STRING
	        - returns the first capture from the regular expression if matched,
	          or the whole match if no captures were specified
`)
}

var verbose = flag.Bool("v", false, "verbose output")

func main() {
	var (
		addr  = flag.String("http", ":8000", "local HTTP `address` to serve the wiki on")
		repo  = flag.String("wiki", "./files", "`directory` with git repository containing wiki files")
		theme = flag.String("theme", "./theme/*.tpl",
			"shell (`glob`) pattern for layout templates (must define 'wiki', see ParseGlob\n"+
				"on https://golang.org/pkg/html/template); rest of files in the directory tree\n"+
				"are served as static assets at /theme/ path")
	)
	flag.Usage = usage
	flag.Parse()

	// Static resources from the theme
	// TODO(akavel): test if this works for static files + for filtering out template files...
	http.Handle("/theme/", http.StripPrefix("/theme/", HttpRejectGlob(filepath.Base(*theme), http.FileServer(http.Dir(filepath.Dir(*theme))))))

	// Main wiki handler
	http.Handle("/", &wikiHandler{
		Repo:         repository(*repo),
		TemplateGlob: *theme,
	})

	log.Printf("Starting a server on %s...", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// HttpRejectGlob returns a 404 Not Found error in case the request path (normalized) matches the glob.
// Otherwise (when glob doesn't match), h is called.
func HttpRejectGlob(glob string, h http.Handler) http.Handler {
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
	urlPath := path.Clean(filepath.ToSlash(r.URL.Path))
	if urlPath == "" || urlPath == "/" {
		p := r.FormValue("path")
		if p != "" && p != "/" && p != "." {
			r.URL.Path = "/" + strings.TrimSuffix(cleanPath(p), ".md")
			http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
			return
		}
	}
	// Don't show any files or directories with names starting with "." (especially ".git")
	for _, segment := range strings.Split(urlPath, "/") {
		if strings.HasPrefix(segment, ".") {
			http.NotFound(w, r)
			return
		}
	}
	// TODO(akavel): make below work also on case-insensitive filesystems
	if strings.HasSuffix(urlPath, ".md") {
		r.URL.Path = strings.TrimSuffix(urlPath, ".md")
		http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
		return
	}
	// If a requested non-.md file exists on disk, return it, under assumption that it is a static resource
	if serveFile(w, r, strings.TrimLeft(urlPath, "/")) {
		return
	}
	switch urlPath {
	case "/favicon.ico":
		http.NotFound(w, r)
		return
	case "/index.html":
		urlPath = "/"
	}

	// Params
	var (
		content   = r.FormValue("content")
		changelog = r.FormValue("msg")
		author    = r.FormValue("author")
		reset     = r.FormValue("revert")
		revision  = r.FormValue("revision")
	)
	query := map[string]string{
		"edit":           r.FormValue("edit"),
		"show_revisions": r.FormValue("show_revisions"),
	}

	node := &node{
		Path: urlPath,
		File: strings.TrimLeft(urlPath, "/") + ".md",
		Dirs: listDirectories(urlPath),
		repo: wiki.Repo,
	}

	switch {
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

func (node *node) gitAdd() *node {
	node.repo.git("add", "--", node.File)
	return node
}

func (node *node) gitCommit(msg string, author string) *node {
	if author != "" {
		node.repo.git("commit", "-m", msg, fmt.Sprintf("--author='%s <system@g-wiki>'", author))
	} else {
		node.repo.git("commit", "-m", msg)
	}
	return node
}

func (node *node) gitShow() *node {
	node.Content = string(node.repo.git("show", node.Revision+":./"+node.File))
	return node
}

func (node *node) gitLog() *node {
	// TODO(akavel): make this configurable?
	const logLimit = "5"
	stdout := node.repo.git("log", "--pretty=format:%h %ad %s", "--date=relative", "-n", logLimit, "--", node.File)
	node.Revisions = nil
	for _, line := range strings.Split(string(stdout), "\n") {
		revision := parseLog(line)
		if revision == nil {
			continue
		}
		node.Revisions = append(node.Revisions, revision)
	}
	if node.Revision == "" && len(node.Revisions) > 0 {
		node.Revision = node.Revisions[0].Hash
	}
	return node
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

type repository string

// git executes a git command with provided arguments.
// Returns nil and logs a message in case of error.
func (repo repository) git(arguments ...string) []byte {
	cmd := exec.Command("git", arguments...)
	cmd.Dir = fmt.Sprintf("%s/", repo)
	if *verbose {
		log.Printf("(wd: %s) %v", cmd.Dir, cmd.Args)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("Error: (%s) command %v failed with:\n%s",
			err, cmd.Args, strings.Join([]string{stdout.String(), stderr.String()}, "\n"))
		return nil
	}
	return stdout.Bytes()
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
			log.Println("glob: %v: %q", err, rawGlob)
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
		node.gitShow()
		nodes = append(nodes, node)
	}
	return nodes
}

func markdown(md string) template.HTML {
	return template.HTML(blackfriday.MarkdownCommon([]byte(md)))
}

func matchre(pattern, s string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	m := re.FindStringSubmatch(s)
	switch len(m) {
	case 0:
		return "", nil
	case 1:
		return m[0], nil
	default:
		return m[1], nil
	}
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
	return strings.TrimLeft(path.Clean(filepath.ToSlash(p)), "/")
}
