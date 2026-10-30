// Package refs reads the references between documents from their markdown. A link to another document's .md file, relative to the folder of the document it is in, is a reference to that document: in the document at gdd/02, [Damage](04.md) refers to gdd/04 and [Index](../index.md) to index.
//
// References are read from the content when they are asked for, never stored, so they cannot fall out of step with the documents.
package refs

import (
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Targets returns the paths of the documents that content, the markdown of the document at from, refers to, in the order they first appear and without repeats. Links inside code are not references, nor are links with a scheme (https:, mailto:), absolute links, links to files that are not .md, and links that leave the store's root.
func Targets(from, content string) []string {
	src := []byte(content)
	doc := goldmark.DefaultParser().Parse(text.NewReader(src))

	var targets []string
	seen := make(map[string]bool)
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		link, ok := n.(*ast.Link)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		if target, ok := resolve(from, string(link.Destination)); ok && !seen[target] {
			seen[target] = true
			targets = append(targets, target)
		}
		return ast.WalkContinue, nil
	}); err != nil {
		// The walker returns no error, so Walk cannot fail.
		return nil
	}
	return targets
}

// resolve turns a link destination in the document at from into the path of the document it names.
func resolve(from, dest string) (string, bool) {
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" || strings.HasPrefix(u.Path, "/") {
		return "", false
	}
	name, ok := strings.CutSuffix(u.Path, ".md")
	if !ok || name == "" {
		return "", false
	}
	target := path.Join(path.Dir(from), name)
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", false
	}
	return target, true
}
