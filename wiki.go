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

// TODO(akavel): add usage help
// TODO(akavel): fix FIXMEs (sanitization of paths, etc.)
// TODO(akavel): allow deleting files from repo
// TODO(akavel): use pure Go git implementation, if such is available
// TODO(akavel): allow adding file attachments into the wiki (images, etc. - probably restrict extensions via flag)
// TODO(akavel): [LATER] nice JS editor, with preview of markdown... but how to ensure compat. with blackfriday? or, VFMD everywhere?.........

var verbose = flag.Bool("v", false, "verbose output")

func main() {
	var (
		addr = flag.String("http", ":8000", "local HTTP `address` to serve the wiki on")
		// repo  = flag.String("wiki", ".", "`directory` with git repository containing wiki files")
		repo = flag.String("wiki", "./files", "`directory` with git repository containing wiki files")
		// theme = flag.String("theme", "./theme/_*.html", "shell (`glob`) pattern for layout templates; "+
		theme = flag.String("theme", "./theme/*.tpl", "shell (`glob`) pattern for layout templates "+
			"(must define 'edit' and 'view', see ParseGlob on https://golang.org/pkg/html/template); "+
			"rest of files in the directory tree are served as static assets at /theme/ path")
	)
	flag.Parse()

	// Static resources from the theme
	// TODO(akavel): test if this works for static files + for filtering out template files...
	http.Handle("/theme/", http.StripPrefix("/theme/", HttpRejectGlob(filepath.Base(*theme), http.FileServer(http.Dir(filepath.Dir(*theme))))))

	// Main wiki handler
	http.Handle("/", &wikiHandler{
		Repo:         *repo,
		TemplateGlob: *theme,
	})

	log.Printf("Starting a server on %s...", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

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
	Repo         string
	TemplateGlob string
}

func (wiki *wikiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := path.Clean(filepath.ToSlash(r.URL.Path))
	// Don't show any files or directories with names starting with "." (especially ".git")
	for _, segment := range strings.Split(urlPath, "/") {
		if strings.HasPrefix(segment, ".") {
			http.NotFound(w, r)
			return
		}
	}
	// If a requested non-.md file exists on disk, return it, under assumption that it is a static resource
	if !strings.HasSuffix(urlPath, ".md") {
		filePath := strings.TrimLeft(urlPath, "/")
		if serveFile(w, r, filePath) {
			return
		}
	}
	switch urlPath {
	case "/favicon.ico":
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
		File: strings.TrimSuffix(strings.TrimLeft(urlPath, "/"), ".md") + ".md",
		Dirs: listDirectories(urlPath),
		Repo: wiki.Repo,
	}

	switch {
	case content != "":
		if changelog == "" {
			changelog = "Update " + node.File
		}
		filePath := filepath.Join(wiki.Repo, node.File)
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
			node.GitAdd().GitCommit(changelog, author).GitLog()
		}
		// TODO(akavel): redirect to normal page, to shake off POST on browser refresh
	case reset != "":
		// Reset to revision
		if *verbose {
			log.Printf("(resetting %q to revision %s)", node.File, reset)
		}
		node.Revision = reset
		node.GitRevert().GitCommit("Reverted to: "+node.Revision, author)
		node.Revision = ""
		node.GitShow().GitLog()
		// TODO(akavel): redirect to normal page, to shake off POST on browser refresh
	default:
		// Show specific revision
		if *verbose {
			log.Printf("(showing %q at revision %s)", node.File, revision)
		}
		node.Revision = revision
		node.GitShow().GitLog()
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
		w.WriteHeader(http.StatusInternalServerError)
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
	Repo string
}

type directory struct {
	Path   string
	Name   string
	Active bool
}

type revision struct {
	Hash    string
	Message string
	Time    string
}

func (node *node) IsHead() bool {
	return len(node.Revisions) > 0 && node.Revision == node.Revisions[0].Hash
}

// Add node
func (node *node) GitAdd() *node {
	gitCmd(exec.Command("git", "add", "--", node.File), node.Repo)
	return node
}

// Commit node message
func (node *node) GitCommit(msg string, author string) *node {
	if author != "" {
		gitCmd(exec.Command("git", "commit", "-m", msg, fmt.Sprintf("--author='%s <system@g-wiki>'", author)), node.Repo)
	} else {
		gitCmd(exec.Command("git", "commit", "-m", msg), node.Repo)
	}
	return node
}

// Fetch node revision
func (node *node) GitShow() *node {
	node.Content = string(gitCmd(exec.Command("git", "show", node.Revision+":./"+node.File), node.Repo))
	return node
}

// Fetch node logFile
func (node *node) GitLog() *node {
	// TODO(akavel): make this configurable?
	const logLimit = "5"
	stdout := gitCmd(exec.Command("git", "log", "--pretty=format:%h %ad %s", "--date=relative", "-n", logLimit, "--", node.File), node.Repo)
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
	s := make([]*directory, 0)
	dirPath := ""
	for i, dir := range strings.Split(path, "/") {
		if i == 0 {
			dirPath += dir
		} else {
			dirPath += "/" + dir
		}
		s = append(s, &directory{Path: dirPath, Name: dir})
	}
	if len(s) > 0 {
		s[len(s)-1].Active = true
	}
	return s
}

// Soft reset to specific revision
func (node *node) GitRevert() *node {
	log.Printf("Reverts %s to revision %s", node.File, node.Revision)
	gitCmd(exec.Command("git", "checkout", node.Revision, "--", node.File), node.Repo)
	return node
}

// Run git command, will currently die on all errors
func gitCmd(cmd *exec.Cmd, baseDirectory string) []byte {
	cmd.Dir = fmt.Sprintf("%s/", baseDirectory)
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

func (node *node) Markdown() template.HTML {
	return template.HTML(blackfriday.MarkdownCommon([]byte(node.Content)))
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
		"query": func() map[string]string { return query },
	}
	t, err := template.New("wiki").Funcs(funcs).ParseGlob(wiki.TemplateGlob)
	if err != nil {
		log.Print("Could not parse template:", err)
		// TODO(akavel): at least print a fallback simple HTML of the node for viewing
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = t.Execute(w, node)
	if err != nil {
		log.Printf("Could not execute template for node %q: %s", node.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
