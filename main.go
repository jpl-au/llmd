/*
Copyright © 2026 James Lawson (jpl-au) <hello@caelisco.net>
*/
package main

import (
	"github.com/jpl-au/llmd/cmd"

	// Import extensions - each registers itself via init()
	_ "github.com/jpl-au/llmd/extension/all"
)

func main() {
	cmd.Execute()
}
