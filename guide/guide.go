// Package guide provides access to embedded documentation pages used by
// the CLI's built-in guide system. Markdown files in this directory are
// compiled into the binary via embed.FS, making them available both as
// repo documentation and at runtime via "llmd guide <topic>".
package guide

import (
	"embed"
	"runtime"
)

//go:embed *.md
var files embed.FS

// Get returns the content of a guide page by name. An empty name returns
// the default overview page. The special name "install" resolves to
// platform-specific instructions based on runtime.GOOS.
func Get(name string) (string, error) {
	if name == "" {
		name = "guide"
	}
	if name == "install" {
		name = "install-" + runtime.GOOS
	}
	data, err := files.ReadFile(name + ".md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// List returns available guide topic names (without the .md suffix).
// The index page (guide.md) is excluded since it is the default.
func List() ([]string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if name != "guide.md" {
			names = append(names, name[:len(name)-3])
		}
	}
	return names, nil
}
