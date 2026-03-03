package config

import "github.com/jpl-au/llmd/sdk"

// Store implements [sdk.ConfigStore] by delegating to the package-level
// functions. Config is store-independent — no host bridge needed.
type Store struct{}

func (Store) Read() (map[string]string, error)                  { return Load() }
func (Store) Write(key, value string, opts sdk.WriteOpts) error { return Save(key, value, opts.Global) }
func (Store) IgnorePatterns() ([]string, error)                 { return IgnorePatterns() }
func (Store) AddIgnore(pattern string) error                    { return AddIgnore(pattern) }
func (Store) RemoveIgnore(pattern string) error                 { return RemoveIgnore(pattern) }
