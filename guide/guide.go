// Package guide provides access to embedded help and guide pages used by
// the CLI's built-in documentation system.
package guide

import (
	"embed"
	"runtime"
)

//go:embed *.md
var files embed.FS

// Get returns the content of a guide page by name. If `name` is empty
// the default "guide" page is returned.
//
// Special case: "install" returns OS-specific instructions based on runtime.GOOS.
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

// List returns the available guide page names (without the .md suffix).
func List() ([]string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if name != "guide.md" {
			names = append(names, name[:len(name)-3]) // strip .md
		}
	}
	return names, nil
}
