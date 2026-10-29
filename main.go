// llmd is a versioned document store for LLMs and humans.
//
// All commands live in extensions. The cli package registers the
// built-in set and cmd.Execute does the rest.
package main

import (
	"os"

	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:]))
}
