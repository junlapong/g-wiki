package main

import (
	"bufio"
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
	"strconv"
	"strings"

	"github.com/russross/blackfriday"
)

// TODO(akavel): add usage help
// TODO(akavel): load .md files from current working directory
// TODO(akavel): ensure that URLs ending with .md are handled properly (redirect to non-.md URLs? but keep #anchors...)
// TODO(akavel): fix FIXMEs (sanitization of paths, etc.)
// TODO(akavel): allow deleting files from repo
// TODO(akavel): (strip .md extension from paths of served files? (+) prettier URLs, more semantic; (-) .md keeps links valid offline !!!!)
// TODO(akavel): use pure Go git implementation, if such is available
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
		// // TODO(akavel): allow dynamic editing, don't cache templates in memory
		// Template: template.Must(template.New("").ParseGlob(*theme)),
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

const (
	logLimit = "5"
)

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
		edit      = r.FormValue("edit")
		changelog = r.FormValue("msg")
		author    = r.FormValue("author")
		reset     = r.FormValue("revert")
		revision  = r.FormValue("revision")
	)

	node := &node{
		Path:          urlPath,
		File:          strings.TrimSuffix(strings.TrimLeft(urlPath, "/"), ".md") + ".md",
		Dirs:          listDirectories(urlPath),
		ShowRevisions: parseBool(r.FormValue("show_revisions")),
		Repo:          wiki.Repo,
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
	case reset != "":
		// Reset to revision
		if *verbose {
			log.Printf("(resetting %q to revision %s)", node.File, reset)
		}
		node.Revision = reset
		node.GitRevert().GitCommit("Reverted to: "+node.Revision, author)
		node.Revision = ""
		node.GitShow().GitLog()
	default:
		// Show specific revision
		if *verbose {
			log.Printf("(showing %q at revision %s)", node.File, revision)
		}
		node.Revision = revision
		node.GitShow().GitLog()
		if edit == "true" || node.Content == "" {
			wiki.renderTemplate(w, "edit", node)
			return
		}
	}
	wiki.renderTemplate(w, "view", node)
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

	Repo string
	// TODO(akavel): move this to a separate template variable/func, like POST or REQUEST in php
	ShowRevisions bool
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
	Link    bool
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
	buf := gitCmd(exec.Command("git", "log", "--pretty=format:%h %ad %s", "--date=relative", "-n", logLimit, "--", node.File), node.Repo)
	var err error
	b := bufio.NewReader(bytes.NewReader(buf))
	var bytes []byte
	node.Revisions = make([]*revision, 0)
	for err == nil {
		bytes, err = b.ReadSlice('\n')
		logLine := parseLog(bytes)
		if logLine == nil {
			continue
		} else if logLine.Hash != node.Revision {
			logLine.Link = true
		}
		node.Revisions = append(node.Revisions, logLine)
	}
	if node.Revision == "" && len(node.Revisions) > 0 {
		node.Revision = node.Revisions[0].Hash
		node.Revisions[0].Link = false
	}
	return node
}

func parseLog(bytes []byte) *revision {
	line := string(bytes)
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

func parseBool(value string) bool {
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return boolValue
}

func writeFile(entry string, bytes []byte) error {
	// FIXME(akavel): make sure to sanitize the 'entry' path
	err := os.MkdirAll(path.Dir(entry), 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(entry, bytes, 0644)
}

func (wiki *wikiHandler) renderTemplate(w http.ResponseWriter, name string, node *node) {
	t, err := template.New("wiki").ParseGlob(wiki.TemplateGlob)
	if err != nil {
		log.Print("Could not parse template:", err)
		// TODO(akavel): at least print a fallback simple HTML of the node for viewing
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = t.ExecuteTemplate(w, name, node)
	if err != nil {
		log.Printf("Could not execute template %q for node %q: %s", name, node.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
