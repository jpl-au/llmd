package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func cat(ctx sdk.Context, args []string) (sdk.Result, error) {
	var paths []string
	var version int
	var numberLines bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-n" {
			numberLines = true
		} else if arg == "--version" && i+1 < len(args) {
			i++
			version, _ = strconv.Atoi(args[i])
		} else if strings.HasPrefix(arg, "--version=") {
			version, _ = strconv.Atoi(strings.TrimPrefix(arg, "--version="))
		} else if !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("cat: missing path argument")
	}

	var results []string
	for _, path := range paths {
		content, err := sdk.API.Read(path, version)
		if err != nil {
			return nil, fmt.Errorf("cat: %s: %w", path, err)
		}

		text := string(content)
		if numberLines {
			text = addLineNumbers(text)
		}
		results = append(results, text)
	}

	output := strings.Join(results, "\n")
	return sdk.Rich{Text: output, Data: output}, nil
}

func addLineNumbers(s string) string {
	lines := strings.Split(s, "\n")
	width := len(strconv.Itoa(len(lines)))
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%*d  %s", width, i+1, line)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
