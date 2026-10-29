// help.go renders a command's own description for the two audiences
// llmd serves. The fields on Command are the single source: Desc,
// Usage and Flags are written once, and the human help, the agent
// help and the MCP tool list are all renderings of them.

package sdk

import (
	"fmt"
	"strings"
)

// Summary returns the first paragraph of Desc: the one or two lines
// that say what the command does, without the explanation that
// follows. It is what listings and agent help show.
func (c *Command) Summary() string {
	text := strings.TrimSpace(c.Desc)
	if i := strings.Index(text, "\n\n"); i >= 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

// AgentHelp returns the command's help for an agent: the text read by
// a model, which pays for every word and asks for detail when it
// needs it. A command with a Brief wrote its own: that text, the
// usage line and where the full documentation is. Any other command
// gets Summary, the usage line, its flags one per line, and the same
// pointer. The prose of Desc that --help shows a person is left out.
//
// The MCP tool list and the --help of a piped llmd both show this.
func (c *Command) AgentHelp() string {
	var b strings.Builder
	if c.Brief != "" {
		b.WriteString(c.Brief)
	} else {
		b.WriteString(c.Summary())
	}
	if c.Usage != "" {
		b.WriteString("\nUsage: llmd " + c.Usage)
	}
	if c.Brief == "" && len(c.Flags) > 0 {
		b.WriteString("\nFlags:")
		for _, f := range c.Flags {
			if f.Short != "" {
				fmt.Fprintf(&b, "\n  -%s, --%s", f.Short, f.Name)
			} else {
				fmt.Fprintf(&b, "\n  --%s", f.Name)
			}
			if f.Type != "" && f.Type != "bool" {
				b.WriteString(" <" + f.Type + ">")
			}
			if f.Desc != "" {
				b.WriteString("  " + f.Desc)
			}
		}
	}
	if page := c.GuidePage(); page != "" {
		fmt.Fprintf(&b, "\nDetails: guide %s", page)
	}
	return b.String()
}

// GuidePage returns the guide page that documents the command, or ""
// when there is none (see Command.Guide).
func (c *Command) GuidePage() string {
	switch c.Guide {
	case "":
		return c.Name
	case "-":
		return ""
	}
	return c.Guide
}
