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
	"regexp"
	"strconv"
	"strings"

	"github.com/russross/blackfriday"
)

// TODO(akavel): add usage help
// TODO(akavel): load .md files from current working directory
// TODO(akavel): load templates & static files from directory specified by flag --theme
// TODO(akavel): also load static files from current working directory, if such exist (override theme's files)
// TODO(akavel): (strip .md extension from paths of served files? (+) prettier URLs, more semantic; (-) .md keeps links valid offline !!!!)
// TODO(akavel): use pure Go git implementation, if such is available
// TODO(akavel): [LATER] nice JS editor, with preview of markdown... but how to ensure compat. with blackfriday? or, VFMD everywhere?.........

func main() {
	var addr = flag.String("http", ":8000", "HTTP `address` to serve the wiki on")
	flag.Parse()

	// Handlers
	http.HandleFunc("/", wikiHandler)
	// Static resources
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	log.Printf("Starting a server on %s...", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

const (
	baseDirectory = "files"
	logLimit      = "5"
)

func wikiHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		return
	}

	// Params
	var (
		content   = r.FormValue("content")
		edit      = r.FormValue("edit")
		changelog = r.FormValue("msg")
		author    = r.FormValue("author")
		reset     = r.FormValue("revert")
		revision  = r.FormValue("revision")
		password  = r.FormValue("password")
	)

	filePath := fmt.Sprintf("%s%s.md", baseDirectory, r.URL.Path)
	node := &node{
		Path:      r.URL.Path,
		File:      r.URL.Path[1:] + ".md",
		Dirs:      listDirectories(r.URL.Path),
		Revisions: parseBool(r.FormValue("revisions")),
		Password:  password,
	}

	if r.URL.Path == "/config" {
		node.Config = true
		if node.accessible() {
			edit = "true"
		} else {
			node.Template = "templates/login.tpl"
		}
	}

	if len(content) != 0 && len(changelog) != 0 && node.accessible() {
		bytes := []byte(content)
		err := writeFile(bytes, filePath)
		if err != nil {
			log.Printf("Can't write to file %s, error: %v", filePath, err)
		} else {
			// Wrote file, commit
			node.Bytes = bytes
			node.GitAdd().GitCommit(changelog, author).GitLog()
			node.ToMarkdown()
		}
	} else if reset != "" && node.accessible() {
		// Reset to revision
		node.Revision = reset
		node.GitRevert().GitCommit("Reverted to: "+node.Revision, author)
		node.Revision = ""
		node.GitShow().GitLog().ToMarkdown()
	} else if node.accessible() {
		// Show specific revision
		node.Revision = revision
		node.GitShow().GitLog()
		if edit == "true" || len(node.Bytes) == 0 {
			node.Content = string(node.Bytes)
			node.Template = "templates/edit.tpl"
		} else {
			node.ToMarkdown()
		}
	}
	renderTemplate(w, node)
}

type node struct {
	Path     string
	File     string
	Content  string
	Template string
	Revision string
	Bytes    []byte
	Dirs     []*directory
	logFile  []*logFile
	Markdown template.HTML

	Revisions bool // Show revisions
	Config    bool // Whether this is a config node
	Password  string
}

type directory struct {
	Path   string
	Name   string
	Active bool
}

type logFile struct {
	Hash    string
	Message string
	Time    string
	Link    bool
}

func (node *node) IsHead() bool {
	return len(node.logFile) > 0 && node.Revision == node.logFile[0].Hash
}

// Add node
func (node *node) GitAdd() *node {
	gitCmd(exec.Command("git", "add", node.File))
	return node
}

// Commit node message
func (node *node) GitCommit(msg string, author string) *node {
	if author != "" {
		gitCmd(exec.Command("git", "commit", "-m", msg, fmt.Sprintf("--author='%s <system@g-wiki>'", author)))
	} else {
		gitCmd(exec.Command("git", "commit", "-m", msg))
	}
	return node
}

// Fetch node revision
func (node *node) GitShow() *node {
	node.Bytes = gitCmd(exec.Command("git", "show", node.Revision+":./"+node.File))
	return node
}

// Fetch node logFile
func (node *node) GitLog() *node {
	buf := gitCmd(exec.Command("git", "log", "--pretty=format:%h %ad %s", "--date=relative", "-n", logLimit, node.File))
	var err error
	b := bufio.NewReader(bytes.NewReader(buf))
	var bytes []byte
	node.logFile = make([]*logFile, 0)
	for err == nil {
		bytes, err = b.ReadSlice('\n')
		logLine := parseLog(bytes)
		if logLine == nil {
			continue
		} else if logLine.Hash != node.Revision {
			logLine.Link = true
		}
		node.logFile = append(node.logFile, logLine)
	}
	if node.Revision == "" && len(node.logFile) > 0 {
		node.Revision = node.logFile[0].Hash
		node.logFile[0].Link = false
	}
	return node
}

func parseLog(bytes []byte) *logFile {
	line := string(bytes)
	re := regexp.MustCompile(`(.{0,7}) (\d+ \w+ ago) (.*)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 4 {
		return &logFile{Hash: matches[1], Time: matches[2], Message: matches[3]}
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
	gitCmd(exec.Command("git", "checkout", node.Revision, "--", node.File))
	return node
}

// Run git command, will currently die on all errors
func gitCmd(cmd *exec.Cmd) []byte {
	cmd.Dir = fmt.Sprintf("%s/", baseDirectory)
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

// ToMarkdown Process node contents
func (node *node) ToMarkdown() {
	node.Markdown = template.HTML(string(blackfriday.MarkdownCommon(node.Bytes)))
}

func (node *node) accessible() bool {
	// TODO(akavel): WTF? rename to .public() and delete the "test" check?
	return !node.Config || node.Password == "test"
}

func parseBool(value string) bool {
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return boolValue
}

func writeFile(bytes []byte, entry string) error {
	// FIXME(akavel): make sure to sanitize the 'entry' path
	err := os.MkdirAll(path.Dir(entry), 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(entry, bytes, 0644)
}

func renderTemplate(w http.ResponseWriter, node *node) {
	t := template.New("wiki")
	var err error

	if node.Template != "" {
		t, err = template.ParseFiles(node.Template)
		if err != nil {
			log.Print("Could not parse template", err)
		}
	} else if node.Markdown != "" {
		t.Parse(`
{{- template "header" . -}}
{{- if .IsHead -}}
	{{- template "actions" . -}}
{{- else if .Revision -}}
	{{- template "revision" . -}}
{{- end -}}
{{- template "node" . -}}
{{- if .Revisions -}}
	{{- template "revisions" . -}}
{{- end -}}
{{- template "footer" . -}}
`)
	}

	// Include the rest
	t.ParseFiles("templates/header.tpl", "templates/footer.tpl",
		"templates/actions.tpl", "templates/revision.tpl",
		"templates/revisions.tpl", "templates/node.tpl")
	err = t.Execute(w, node)
	if err != nil {
		log.Print("Could not execute template: ", err)
	}
}
