// Package guide provides embedded documentation pages that ship with
// the llmd binary. Each topic is a markdown file in this directory,
// accessible via [Get] and [List]. An extension adds its own pages
// with [Add].
package guide

import (
	"embed"
	"fmt"
	"io/fs"
	"runtime"
	"slices"
	"strings"
	"sync"
)

//go:embed *.md
var files embed.FS

var (
	mu sync.RWMutex
	// added holds the pages extensions added, by name.
	added = map[string][]byte{}
)

// Add makes every markdown file at the top of fsys a guide page, named
// after the file without .md, so that "llmd guide <name>" shows it and
// a command's help can point to it. An extension calls it from init,
// with the pages it embeds beside its code:
//
//	//go:embed *.md
//	var pages embed.FS
//
//	func init() {
//		guide.Add(pages)
//	}
//
// Add panics when fsys cannot be read or holds a page that llmd or
// another extension already has, as extension.Register does for a
// name already taken: two pages under one name would leave one of
// them unreachable.
func Add(fsys fs.FS) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		panic(fmt.Sprintf("guide: reading the pages to add: %v", err))
	}
	mu.Lock()
	defer mu.Unlock()
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if e.IsDir() || !ok {
			continue
		}
		if _, err := files.ReadFile(e.Name()); err == nil {
			panic("guide: llmd already has a guide page " + name)
		}
		if _, dup := added[name]; dup {
			panic("guide: a guide page " + name + " was already added")
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			panic(fmt.Sprintf("guide: reading %s: %v", e.Name(), err))
		}
		added[name] = data
	}
}

// Get returns the content of a guide page by name. An empty name
// returns the default overview page. The special name "install"
// resolves to platform-specific instructions based on runtime.GOOS.
func Get(name string) (string, error) {
	if name == "" {
		name = "guide"
	}
	if name == "install" {
		name = "install-" + runtime.GOOS
	}
	mu.RLock()
	data, ok := added[name]
	mu.RUnlock()
	if ok {
		return string(data), nil
	}
	data, err := files.ReadFile(name + ".md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// List returns available guide topic names (without the .md suffix),
// llmd's own and those extensions added, in order. The index page
// (guide.md) is excluded since it is the default.
func List() ([]string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if name != "guide.md" && !e.IsDir() {
			names = append(names, name[:len(name)-3])
		}
	}
	mu.RLock()
	for name := range added {
		names = append(names, name)
	}
	mu.RUnlock()
	slices.Sort(names)
	return names, nil
}
