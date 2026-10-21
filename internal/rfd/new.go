package rfd

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

//go:embed template
var templateFiles embed.FS

// Template is the default template a new RFD starts from, for projects
// without their own.
var Template, _ = fs.Sub(templateFiles, "template")

// A Draft is the content of a new RFD.
type Draft struct {
	Title   string
	Authors string // "Name <address>", comma-separated
	Labels  string // comma-separated, starting with the project
}

// Create starts the next RFD in dir from the README.adoc and
// IMPLEMENTATION.adoc templates in templates. It returns the new RFD's
// directory name.
func Create(dir string, templates fs.FS, d Draft) (string, error) {
	if d.Title == "" || strings.ContainsAny(d.Title, "\r\n") {
		return "", fmt.Errorf("an RFD title must be one non-empty line")
	}
	names, err := DirTree(dir).ReadDir(".")
	if err != nil {
		return "", fmt.Errorf("RFD directory not found: %s", dir)
	}
	number := 1
	for _, name := range names {
		if n, err := strconv.Atoi(name); err == nil && entryName.MatchString(name) {
			number = max(number, n+1)
		}
	}
	entry := fmt.Sprintf("%04d", number)
	data := struct {
		Draft
		Number int
		Entry  string
	}{d, number, entry}

	files := map[string][]byte{}
	for _, name := range []string{"README.adoc", "IMPLEMENTATION.adoc"} {
		t, err := template.New(name).Option("missingkey=error").ParseFS(templates, name)
		if err != nil {
			return "", err
		}
		var b bytes.Buffer
		if err := t.Execute(&b, data); err != nil {
			return "", err
		}
		files[name] = b.Bytes()
	}

	path := filepath.Join(dir, entry)
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(path, name), content, 0o644); err != nil {
			return "", err
		}
	}
	return entry, nil
}
