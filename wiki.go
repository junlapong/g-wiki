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

const (
	baseDirectory = "files"
	logLimit      = "5"
)

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

func (node *node) isHead() bool {
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
	buf := gitCmd(exec.Command("git", "show", node.Revision+":"+node.File))
	node.Bytes = buf.Bytes()
	return node
}

// Fetch node logFile
func (node *node) GitLog() *node {
	buf := gitCmd(exec.Command("git", "logFile", "--pretty=format:%h %ad %s", "--date=relative", "-n", logLimit, node.File))
	var err error
	b := bufio.NewReader(buf)
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
func gitCmd(cmd *exec.Cmd) *bytes.Buffer {
	cmd.Dir = fmt.Sprintf("%s/", baseDirectory)
	var out bytes.Buffer
	cmd.Stdout = &out
	runError := cmd.Run()
	if runError != nil {
		fmt.Printf("Error: (%s) command failed with:\n\"%s\n\"", runError, out.String())
		return bytes.NewBuffer([]byte{})
	}
	return &out
}

// ToMarkdown Process node contents
func (node *node) ToMarkdown() {
	node.Markdown = template.HTML(string(blackfriday.MarkdownCommon(node.Bytes)))
}

func (node *node) accessible() bool {
	return (!node.Config || node.Password == "test")
}

// ParseBool given a string, convert to bolean
func ParseBool(value string) bool {
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return boolValue
}

func wikiHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		return
	}

	// Params
	content := r.FormValue("content")
	edit := r.FormValue("edit")
	changelog := r.FormValue("msg")
	author := r.FormValue("author")
	reset := r.FormValue("revert")
	revision := r.FormValue("revision")
	password := r.FormValue("password")

	filePath := fmt.Sprintf("%s%s.md", baseDirectory, r.URL.Path)
	node := &node{File: r.URL.Path[1:] + ".md", Path: r.URL.Path, Password: password}
	node.Revisions = ParseBool(r.FormValue("revisions"))

	node.Dirs = listDirectories(r.URL.Path)
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

		if err := writeFile(bytes, filePath); err != nil {
			log.Printf("Cant write to file %s, error: %v", filePath, err)
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

func writeFile(bytes []byte, entry string) error {
	err := os.MkdirAll(path.Dir(entry), 0777)
	if err == nil {
		return ioutil.WriteFile(entry, bytes, 0644)
	}
	return err
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
		tpl := "{{ template \"header\" . }}"
		if node.isHead() {
			tpl += "{{ template \"actions\" .}}"
		} else if node.Revision != "" {
			tpl += "{{ template \"revision\" . }}"
		}
		// Add node
		tpl += "{{ template \"node\" . }}"
		// Show revisions
		if node.Revisions {
			tpl += "{{ template \"revisions\" . }}"
		}

		// Footer
		tpl += "{{ template \"footer\" . }}"
		t.Parse(tpl)
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

func main() {
	// Handlers
	http.HandleFunc("/", wikiHandler)

	// Static resources
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	var local = flag.String("local", "", "serve as webserver, example: 0.0.0.0:8000")

	flag.Parse()
	var err error

	if *local != "" {
		err = http.ListenAndServe(*local, nil)
	}
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
