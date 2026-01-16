// config_keys.go provides key-value access to configuration settings.
//
// Separated from config.go to isolate the key enumeration and string-based
// get/set logic. This separation allows config.go to focus on YAML structure
// and loading, while this file handles the MCP and CLI interface where config
// is accessed by string keys (e.g., "limits.max_content").
//
// Design: Pointers are used for optional fields so we can distinguish between
// "not set" (nil) and "explicitly set to zero/false". This enables proper
// defaulting - we only apply defaults when the user hasn't set a value.

package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ValidKeys returns all valid configuration keys.
func ValidKeys() []string {
	return []string{
		"author.name", "author.email",
		"sync.files",
		"limits.max_path", "limits.max_content", "limits.max_line_length",
	}
}

// IsValidKey returns true if the key is a valid configuration key.
func IsValidKey(key string) bool {
	return slices.Contains(ValidKeys(), key)
}

// Get returns the value of a configuration key as a string.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "author.name":
		return c.Author.Name, nil
	case "author.email":
		return c.Author.Email, nil
	case "sync.files":
		if c.SyncFiles() {
			return "true", nil
		}
		return "false", nil
	case "limits.max_path":
		return strconv.Itoa(c.MaxPath()), nil
	case "limits.max_content":
		return strconv.FormatInt(c.MaxContent(), 10), nil
	case "limits.max_line_length":
		return strconv.Itoa(c.MaxLineLength()), nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}
}

// Set sets the value of a configuration key.
func (c *Config) Set(key, value string) error {
	switch key {
	case "author.name":
		c.Author.Name = value
	case "author.email":
		c.Author.Email = value
	case "sync.files":
		v := strings.ToLower(value)
		if v != "true" && v != "false" {
			return fmt.Errorf("%w: sync.files must be true or false", ErrInvalidValue)
		}
		b := v == "true"
		c.Sync.Files = &b
	case "limits.max_path":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("%w: limits.max_path must be a positive integer", ErrInvalidValue)
		}
		c.Limits.MaxPath = &n
	case "limits.max_content":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n <= 0 {
			return fmt.Errorf("%w: limits.max_content must be a positive integer", ErrInvalidValue)
		}
		c.Limits.MaxContent = &n
	case "limits.max_line_length":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("%w: limits.max_line_length must be a positive integer", ErrInvalidValue)
		}
		c.Limits.MaxLineLength = &n
	default:
		return fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}
	return nil
}

// All returns all configuration values as a map.
func (c *Config) All() map[string]string {
	return map[string]string{
		"author.name":            c.Author.Name,
		"author.email":           c.Author.Email,
		"sync.files":             strconv.FormatBool(c.SyncFiles()),
		"limits.max_path":        strconv.Itoa(c.MaxPath()),
		"limits.max_content":     strconv.FormatInt(c.MaxContent(), 10),
		"limits.max_line_length": strconv.Itoa(c.MaxLineLength()),
	}
}

// IsSet returns true if the key has an explicit value (not just defaults).
func (c *Config) IsSet(key string) bool {
	switch key {
	case "author.name":
		return c.Author.Name != ""
	case "author.email":
		return c.Author.Email != ""
	case "sync.files":
		return c.Sync.Files != nil
	case "limits.max_path":
		return c.Limits.MaxPath != nil
	case "limits.max_content":
		return c.Limits.MaxContent != nil
	case "limits.max_line_length":
		return c.Limits.MaxLineLength != nil
	default:
		return false
	}
}
