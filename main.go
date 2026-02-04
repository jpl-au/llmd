package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jpl-au/llmd/cli" // registers cli extension
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printHelp(host.New(nil))
		return 0
	}

	// Parse global flags and extract command
	var jsonOut bool
	var help bool
	var cmd string
	var cmdArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--help" || arg == "-h":
			help = true
		case cmd == "" && !strings.HasPrefix(arg, "-"):
			cmd = arg
			cmdArgs = args[i+1:]
			i = len(args)
		}
	}

	// Built-in commands
	switch cmd {
	case "version":
		fmt.Println("llmd dev")
		return 0

	case "init":
		if help {
			fmt.Println("init - Initialize a new store\n\nUsage: llmd init")
			return 0
		}
		store, err := llmd.Init("")
		if err != nil {
			return errorf(jsonOut, "init: %v", err)
		}
		store.Close()
		fmt.Println("Initialized llmd store in .llmd/")
		return 0

	case "config":
		if help {
			fmt.Println(`config - Manage configuration

Usage:
  llmd config                       Show all config
  llmd config <key>                 Show specific key
  llmd config <key> <value>         Set config value

Keys:
  author    Your name for document authorship`)
			return 0
		}
		return runConfig(cmdArgs, jsonOut)

	case "vacuum":
		if help {
			fmt.Println("vacuum - Clean up deleted documents\n\nUsage: llmd vacuum")
			return 0
		}
		store, err := llmd.Open("")
		if err != nil {
			return errorf(jsonOut, "vacuum: %v", err)
		}
		defer store.Close()
		fmt.Println("Vacuum complete")
		return 0

	case "plugins":
		if help {
			fmt.Println("plugins - List loaded plugins\n\nUsage: llmd plugins")
			return 0
		}
		h := host.New(nil)
		for _, p := range h.Plugins() {
			fmt.Printf("%s\n", p.Name())
		}
		return 0
	}

	// Help for no command
	if cmd == "" || help && cmd == "" {
		h := host.New(nil)
		printHelp(h)
		return 0
	}

	// Plugin commands need store
	store, err := llmd.Open("")
	if err != nil {
		return errorf(jsonOut, "%v", err)
	}
	defer store.Close()

	h := host.New(store)

	// Command help
	if help {
		c := h.Commands()[cmd]
		if c == nil {
			return errorf(jsonOut, "unknown command: %s", cmd)
		}
		printCmdHelp(c)
		return 0
	}

	// Check command exists
	if h.Commands()[cmd] == nil {
		return errorf(jsonOut, "unknown command: %s", cmd)
	}

	// Load author from config
	author := loadConfig()["author"]
	if author == "" && needsAuthor(cmd) {
		return errorf(jsonOut, "author not configured\n\nSet your author name:\n  llmd config author \"Your Name\"")
	}

	// Read stdin
	stdin := readStdin()

	// Execute
	result, err := h.Exec(cmd, cmdArgs, author, stdin)
	if err != nil {
		return errorf(jsonOut, "%v", err)
	}

	// Output
	switch r := result.(type) {
	case sdk.Text:
		if string(r) != "" {
			fmt.Println(string(r))
		}
	case sdk.Rich:
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(r.Data)
		} else if r.Text != "" {
			fmt.Println(r.Text)
		}
	case sdk.Data:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r.V)
	}

	return 0
}

func runConfig(args []string, jsonOut bool) int {
	cfg := loadConfig()

	switch len(args) {
	case 0:
		// Show all
		for k, v := range cfg {
			fmt.Printf("%s=%s\n", k, v)
		}
		return 0

	case 1:
		// Show specific key
		if v, ok := cfg[args[0]]; ok {
			fmt.Println(v)
		}
		return 0

	case 2:
		// Set key=value
		key, value := args[0], args[1]
		if key != "author" {
			return errorf(jsonOut, "unknown config key: %s", key)
		}
		if err := saveConfig(key, value); err != nil {
			return errorf(jsonOut, "saving config: %v", err)
		}
		return 0

	default:
		return errorf(jsonOut, "usage: llmd config [key] [value]")
	}
}

func configPath() string {
	// Local config in .llmd/config takes precedence
	if _, err := os.Stat(".llmd/config"); err == nil {
		return ".llmd/config"
	}
	// Fall back to global config
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "llmd", "config")
}

func loadConfig() map[string]string {
	cfg := make(map[string]string)

	// Load global config first
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".config", "llmd", "config")
	loadConfigFile(globalPath, cfg)

	// Local config overrides
	loadConfigFile(".llmd/config", cfg)

	return cfg
}

func loadConfigFile(path string, cfg map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			cfg[key] = value
		}
	}
}

func saveConfig(key, value string) error {
	path := ".llmd/config"

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Load existing config
	cfg := make(map[string]string)
	loadConfigFile(path, cfg)
	cfg[key] = value

	// Write back
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for k, v := range cfg {
		fmt.Fprintf(f, "%s=%s\n", k, v)
	}
	return nil
}

func errorf(jsonOut bool, format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	if jsonOut {
		json.NewEncoder(os.Stderr).Encode(map[string]string{"error": msg})
	} else {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	return 1
}

func needsAuthor(cmd string) bool {
	switch cmd {
	case "write", "edit", "rm", "mv", "restore", "revert":
		return true
	}
	return false
}

func readStdin() []byte {
	f := os.Stdin
	stat, err := f.Stat()
	if err != nil {
		return nil
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	if stat.Mode().IsRegular() {
		data, _ := io.ReadAll(f)
		return data
	}
	done := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(f)
		done <- data
	}()
	select {
	case data := <-done:
		return data
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}

func printHelp(h *host.Host) {
	fmt.Print(`llmd - a document store for LLMs and humans

Usage:
  llmd <command> [flags] [args...]

Global Flags:
  --json              Output as JSON
  --help              Show help

Built-in Commands:
  config      Manage configuration
  init        Initialize a new store
  plugins     List loaded plugins
  vacuum      Clean up deleted documents
  version     Show version information

`)
	if h != nil {
		cmds := h.Commands()
		if len(cmds) > 0 {
			fmt.Println("Plugin Commands:")
			names := make([]string, 0, len(cmds))
			for name := range cmds {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf("  %-12s%s\n", name, cmds[name].Desc)
			}
			fmt.Println()
		}
	}
	fmt.Println(`Use "llmd <command> --help" for more information about a command.`)
}

func printCmdHelp(c *sdk.Command) {
	fmt.Printf("%s - %s\n\n", c.Name, c.Desc)
	if c.Usage != "" {
		fmt.Printf("Usage:\n  llmd %s\n\n", c.Usage)
	}
	if len(c.Flags) > 0 {
		fmt.Println("Flags:")
		for _, f := range c.Flags {
			var flagStr string
			if f.Short != "" {
				flagStr = fmt.Sprintf("-%s, --%s", f.Short, f.Name)
			} else {
				flagStr = fmt.Sprintf("    --%s", f.Name)
			}
			switch f.Type {
			case "string":
				flagStr += " <string>"
			case "int":
				flagStr += " <int>"
			}
			fmt.Printf("  %-24s%s\n", flagStr, f.Desc)
		}
		fmt.Println()
	}
	fmt.Println("Global Flags:")
	fmt.Println("  --json              Output as JSON")
	fmt.Println("  --help              Show help")
}
