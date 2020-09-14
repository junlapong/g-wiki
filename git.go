package main

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

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
