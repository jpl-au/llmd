/*
Copyright © 2026 James Lawson (jpl-au) <hello@caelisco.net>
*/

// init_extensions.go handles extension initialisation and command registration.
//
// Separated from root.go to isolate the complex initialisation logic that
// discovers the store, loads config, and wires up extensions.
//
// Design: Extensions register during init() but aren't initialised until
// first command execution. This two-phase pattern allows extensions to
// declare commands before the store exists. The service is created once
// and shared across all extensions via the Context.

package cmd

import (
	"fmt"
	"sync"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/log"
)

// noStoreCommands lists commands that bypass automatic store initialisation.
// Built dynamically from bootstrap commands plus extension-declared storeless commands.
var noStoreCommands map[string]bool

// authorRequiredCommands lists commands that require author configuration.
// These are commands that write or modify document data.
var authorRequiredCommands = map[string]bool{
	"write":   true,
	"edit":    true,
	"sed":     true,
	"rm":      true,
	"mv":      true,
	"revert":  true,
	"restore": true,
	"import":  true,
	"sync":    true,
	"tag":     true,
	"link":    true,
	"unlink":  true,
	"vacuum":  true,
}

// buildNoStoreCommands creates the set of commands that skip store initialisation.
//
// Why this exists: Most commands need the document store, but some must work
// without it. There are two categories:
//
//  1. Bootstrap commands (init, guide, config, llm) - These help users set up
//     or learn about llmd before a store exists. Running "llmd guide" shouldn't
//     fail just because you haven't run "llmd init" yet.
//
//  2. Extension-declared storeless commands - Extensions can implement the
//     Storeless interface to declare commands that manage their own service
//     lifecycle. For example, "import --dry-run" must work without a store.
//
// When adding a new command: If it's a core bootstrap command, add it here.
// Otherwise, implement extension.Storeless in your extension.
func buildNoStoreCommands() map[string]bool {
	cmds := map[string]bool{
		// Core bootstrap commands - always storeless
		"init":   true,
		"guide":  true,
		"config": true,
		"llm":    true,
	}

	// Add extension-declared storeless commands
	for _, ext := range extension.All() {
		if s, ok := ext.(extension.Storeless); ok {
			for _, name := range s.NoStoreCommands() {
				cmds[name] = true
			}
		}
	}

	return cmds
}

// Global extension context, created during initialisation.
var (
	extContext extension.Context
	extService *document.Service
	initOnce   sync.Once
	initErr    error
)

// initExtensions creates the document service and injects it into extensions.
//
// Why sync.Once: The service is expensive to create (opens DB, sets up WAL mode)
// and must be shared across all extensions. We use sync.Once to guarantee exactly
// one initialisation per process, even if multiple commands somehow trigger it.
//
// Error handling: ErrNotInitialised is expected for first-time users who haven't
// run "llmd init" yet - we skip silently and let the command fail with a clear
// message. Other errors (permissions, corruption) are returned immediately.
func initExtensions() error {
	initOnce.Do(func() {
		svc, err := document.New(DB())
		if err != nil {
			initErr = fmt.Errorf("opening database: %w", err)
			return
		}
		extService = svc

		// Set project identifier for audit logging
		log.SetProject(svc.FilesDir())

		cfg, err := config.Load()
		if err != nil {
			initErr = err
			return
		}
		extContext = extension.NewContext(svc, svc.DB(), cfg)
		svc.SetExtensionContext(extContext)

		// Inject the shared context into all Initializable extensions.
		// This is dependency injection - extensions receive the service rather
		// than creating it themselves, enabling shared state and proper cleanup.
		for _, ext := range extension.All() {
			if init, ok := ext.(extension.Initializable); ok {
				if err := init.Init(extContext); err != nil {
					initErr = fmt.Errorf("init extension %s: %w", ext.Name(), err)
					return
				}
			}
		}
	})
	return initErr
}

var extensionsOnce sync.Once

// registerExtensions adds commands from all registered extensions.
// Called once before Execute runs.
func registerExtensions() {
	extensionsOnce.Do(func() {
		for _, ext := range extension.All() {
			for _, cmd := range ext.Commands() {
				rootCmd.AddCommand(cmd)
			}
		}

		// Build noStoreCommands after all extensions are registered
		noStoreCommands = buildNoStoreCommands()
	})
}
