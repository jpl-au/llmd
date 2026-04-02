// Package rules manages column transition and automation rules.
//
// Rules are stored as YAML files in .llmd/rules/. Each file defines
// per-column behaviour: where tasks go on success or failure, and
// optionally which agent handles the work. Columns without an agent
// entry are manual - a human triggers the work, but the transitions
// still follow the rule.
//
//	.llmd/rules/
//	  default.yaml     default rule set (created on first use)
package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/config"
	"gopkg.in/yaml.v3"
)

// ColumnRule defines the behaviour for a single board column.
type ColumnRule struct {
	Agent   string `yaml:"agent,omitempty"`
	Role    string `yaml:"role,omitempty"`
	Success string `yaml:"success"`
	Failure string `yaml:"failure"`
	Resume  bool   `yaml:"resume,omitempty"`
}

// RuleSet maps column names to their rules.
type RuleSet map[string]ColumnRule

// Column returns the rule for a column. Returns a zero-value
// ColumnRule if the column has no explicit rule.
func (rs RuleSet) Column(name string) ColumnRule {
	return rs[name]
}

// Load reads a rule set from <dir>/rules/<name>.yaml. If the
// default rule file does not exist, it is created with standard
// transitions on first access (lazy initialisation).
func Load(dir, name string) (RuleSet, error) {
	path := filepath.Join(dir, "rules", name+".yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if name == "default" {
			if err := Seed(dir); err != nil {
				return nil, err
			}
			return Default(), nil
		}
		return RuleSet{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading rules: %w", err)
	}

	var rs RuleSet
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("parsing rules: %w", err)
	}
	return rs, nil
}

// Save writes a rule set to <dir>/rules/<name>.yaml.
func Save(dir, name string, rs RuleSet) error {
	rulesDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("creating rules directory: %w", err)
	}

	data, err := yaml.Marshal(rs)
	if err != nil {
		return fmt.Errorf("encoding rules: %w", err)
	}

	path := filepath.Join(rulesDir, name+".yaml")
	return os.WriteFile(path, data, 0644)
}

// Default returns the standard manual transitions for the default
// board columns. No agents are configured - all columns are manual.
// Failure transitions go to "blocked" (human intervention) rather
// than looping back to the same column.
func Default() RuleSet {
	return RuleSet{
		"up-next": {
			Success: "in-progress",
			Failure: "up-next",
		},
		"in-progress": {
			Success: "review",
			Failure: "blocked",
		},
		"review": {
			Success: "approval",
			Failure: "blocked",
		},
	}
}

// Seed creates the default rule file if it does not exist and
// whitelists the rules directory in .gitignore.
func Seed(dir string) error {
	path := filepath.Join(dir, "rules", "default.yaml")
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := Save(dir, "default", Default()); err != nil {
		return err
	}

	return config.GitAllow("rules/")
}

// Sync updates the default rule file to match the current board
// columns. New columns get empty rule entries; removed columns
// have their entries deleted.
func Sync(dir string, cols []string) error {
	rs, err := Load(dir, "default")
	if err != nil {
		return err
	}

	// Remove entries for columns no longer on the board.
	all := make(map[string]bool, len(cols))
	for _, c := range cols {
		all[c] = true
	}
	for col := range rs {
		if !all[col] {
			delete(rs, col)
		}
	}

	return Save(dir, "default", rs)
}
