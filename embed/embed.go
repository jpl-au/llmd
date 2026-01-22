// Package embed provides embedded assets for llmd.
package embed

import (
	_ "embed"
)

// CorePlugin contains the compiled core plugin WASM binary.
// This is embedded at build time by go:embed.
//
//go:embed core.wasm
var CorePlugin []byte
