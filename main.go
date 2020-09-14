package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var verbose = flag.Bool("v", false, "verbose output")

func main() {

	addr := flag.String("http", ":8000", "local HTTP `address` to serve the wiki on")
	repo := flag.String("wiki", "./files", "`directory` with git repository containing wiki files")
	templates := "./templates/*.tpl"

	flag.Usage = usage
	flag.Parse()

	// Static resources from the templates
	// TODO(akavel): test if this works for static files + for filtering out template files...
	http.Handle("/templates/", http.StripPrefix("/templates/", HTTPRejectGlob(filepath.Base(templates),
		http.FileServer(http.Dir(filepath.Dir(templates))))))

	// Main wiki handler
	http.Handle("/", &wikiHandler{
		Repo:         repository(*repo),
		TemplateGlob: templates,
	})

	log.Printf("Starting a server on %s...", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func usage() {
	fmt.Fprintf(os.Stderr, `
Simple wiki service using git as the storage back-end.
Content is formatted in markdown syntax.

WARNING: the wiki has no protections against malicious editing,
and no support for multiple simultaneous editors.

Usage:

 %s [flags]

flags:
`, os.Args[0])
	flag.PrintDefaults()
}
