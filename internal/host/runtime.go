// This file sets up the WebAssembly runtime for loading and executing plugins.
//
// The runtime uses wazero, a pure Go WebAssembly runtime, to execute plugin
// WASM modules. Each plugin runs in its own isolated WASM instance with:
//
//   - WASI (WebAssembly System Interface): Provides basic system call emulation
//     for things like reading environment variables and basic I/O.
//
//   - Host functions: Custom functions exported from the host that plugins call
//     to access the document store. These are defined in the proto/host package
//     and implemented by HostFuncs.
//
// # Module Lifecycle
//
// Plugins are built with -buildmode=c-shared, which creates "reactor" modules.
// Unlike "command" modules that execute main() and exit, reactor modules:
//
//  1. Export an _initialize function (not _start)
//  2. Initialise global state and run init() functions
//  3. Stay alive after initialisation, allowing the host to call exports
//
// This is why plugin registration uses init() rather than main().
package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/debug"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/paths"
	hostpb "github.com/jpl-au/llmd/proto/host"
	"github.com/jpl-au/llmd/proto/plugin"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime manages the wazero runtime and plugin loading.
//
// The Runtime creates a wazero runtime configured with WASI support and the
// llmd host functions. It provides a plugin loader that can load WASM modules
// and interact with them via the proto-generated interface.
type Runtime struct {
	// store is the llmd document store.
	store *llmd.Store

	// hostFuncs implements the host functions that plugins can call.
	hostFuncs *HostFuncs

	// loader is the go-plugin loader for instantiating WASM plugins.
	loader *plugin.PluginPlugin
}

// NewRuntime creates a new plugin runtime.
//
// The runtime is configured with:
//  1. WASI support for basic system call emulation
//  2. Host functions that allow plugins to access the document store
//
// The go-plugin library uses a custom wazero runtime factory so we can inject
// our host functions into the "env" module before loading plugins.
//
// The store parameter can be nil for discovery commands that only need plugin
// manifests. Host functions will return ErrStoreNotAvailable if called with
// a nil store.
//
// Plugins built with -buildmode=c-shared export _initialize rather than _start,
// which is the default mode for go-plugin, so no special configuration is needed.
func NewRuntime(ctx context.Context, store *llmd.Store) (*Runtime, error) {
	debug.Log("NewRuntime", "storeAvailable", store != nil)

	hostFuncs := NewHostFuncs(store)

	// Set up compilation cache to speed up subsequent runs.
	// Compiled WASM is cached globally (~/.llmd/cache on Unix, %LOCALAPPDATA%\llmd\cache
	// on Windows) so we don't recompile every time.
	// The cache is keyed by WASM content, so changed plugins get new entries.
	// Old cache entries are not automatically cleaned up - delete the cache directory
	// to clear if it grows too large.
	cacheDir, err := paths.CacheDir()
	if err != nil {
		debug.Log("NewRuntime error", "step", "cacheDir", "error", err.Error())
		return nil, err
	}
	debug.Log("NewRuntime", "cacheDir", cacheDir)

	cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
	if err != nil {
		debug.Log("NewRuntime error", "step", "cache", "error", err.Error())
		return nil, err
	}
	runtimeConfig := wazero.NewRuntimeConfig().WithCompilationCache(cache)

	// Create plugin loader with custom runtime that includes our host functions.
	// The WazeroRuntime option lets us configure the runtime before plugins load.
	loader, err := plugin.NewPluginPlugin(ctx,
		plugin.WazeroRuntime(func(ctx context.Context) (wazero.Runtime, error) {
			debug.Log("WazeroRuntime factory called")
			r := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

			// Instantiate WASI - provides basic system call emulation (env vars, args, etc.)
			if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
				debug.Log("WazeroRuntime error", "step", "WASI", "error", err.Error())
				r.Close(ctx)
				return nil, err
			}
			debug.Log("WazeroRuntime", "step", "WASI instantiated")

			// Instantiate our host functions - allows plugins to access the document store
			if err := hostpb.Instantiate(ctx, r, hostFuncs); err != nil {
				debug.Log("WazeroRuntime error", "step", "hostFuncs", "error", err.Error())
				r.Close(ctx)
				return nil, err
			}
			debug.Log("WazeroRuntime", "step", "hostFuncs instantiated")

			return r, nil
		}),
	)
	if err != nil {
		debug.Log("NewRuntime error", "step", "pluginLoader", "error", err.Error())
		return nil, err
	}

	debug.Log("NewRuntime complete")
	return &Runtime{
		store:     store,
		hostFuncs: hostFuncs,
		loader:    loader,
	}, nil
}

// Loader returns the plugin loader for loading WASM plugin modules.
//
// The loader's Load method takes a file path and returns a plugin instance
// that can be initialised and used to execute commands.
func (r *Runtime) Loader() *plugin.PluginPlugin {
	return r.loader
}

// Store returns the underlying llmd document store.
func (r *Runtime) Store() *llmd.Store {
	return r.store
}
