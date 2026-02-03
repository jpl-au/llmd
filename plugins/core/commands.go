package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

func ls(ctx sdk.Context, args []string) (sdk.Result, error) {
	var long, all, reverse, sortByTime bool
	var prefix string

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			for _, c := range arg[1:] {
				switch c {
				case 'l':
					long = true
				case 'a':
					all = true
				case 'r':
					reverse = true
				case 't':
					sortByTime = true
				}
			}
		} else if !strings.HasPrefix(arg, "-") {
			prefix = arg
		}
	}

	sort := "path"
	if sortByTime {
		sort = "time"
	}

	docs, err := sdk.API.List(prefix, sdk.ListOpts{
		Deleted: all,
		Sort:    sort,
		Reverse: reverse,
	})
	if err != nil {
		return nil, err
	}

	if len(docs) == 0 {
		return sdk.Rich{Text: "", Data: []sdk.Doc{}}, nil
	}

	data := make([]map[string]any, len(docs))
	for i, d := range docs {
		data[i] = map[string]any{
			"path":       d.Path,
			"version":    d.Version,
			"author":     d.Author,
			"message":    d.Message,
			"created_at": d.CreatedAt,
			"deleted":    d.Deleted,
		}
	}

	var text string
	if long {
		text = formatTable(docs)
	} else {
		paths := make([]string, len(docs))
		for i, d := range docs {
			paths[i] = d.Path
		}
		text = strings.Join(paths, "\n")
	}

	return sdk.Rich{Text: text, Data: data}, nil
}

func formatTable(docs []sdk.Doc) string {
	if len(docs) == 0 {
		return ""
	}

	verWidth := 3
	authorWidth := 6
	for _, d := range docs {
		if w := len(fmt.Sprintf("%d", d.Version)); w > verWidth {
			verWidth = w
		}
		if w := len(d.Author); w > authorWidth {
			authorWidth = w
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%*s  %-*s  %-10s  %s\n", verWidth, "VER", authorWidth, "AUTHOR", "DATE", "PATH")

	for _, d := range docs {
		date := time.UnixMilli(d.CreatedAt).Format("2006-01-02")
		path := d.Path
		if d.Deleted {
			path = d.Path + " (deleted)"
		}
		fmt.Fprintf(&b, "%*d  %-*s  %-10s  %s\n", verWidth, d.Version, authorWidth, d.Author, date, path)
	}

	return strings.TrimSuffix(b.String(), "\n")
}

func write(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("write: missing path argument")
	}

	path := args[0]
	var message string
	for i := 1; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if strings.HasPrefix(args[i], "--message=") {
			message = strings.TrimPrefix(args[i], "--message=")
		}
	}

	if err := sdk.API.Write(path, ctx.Stdin, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Wrote %s", path)), nil
}

func rm(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("rm: missing path argument")
	}

	if err := sdk.API.Delete(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("rm: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Deleted %s", args[0])), nil
}

func mv(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("mv: requires <from> <to> arguments")
	}

	if err := sdk.API.Move(args[0], args[1], ctx.Author); err != nil {
		return nil, fmt.Errorf("mv: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved %s -> %s", args[0], args[1])), nil
}

func edit(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("edit: requires <path> <old> <new> arguments")
	}

	path, old, new := args[0], args[1], args[2]

	var message string
	for i := 3; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if strings.HasPrefix(args[i], "--message=") {
			message = strings.TrimPrefix(args[i], "--message=")
		}
	}

	if err := sdk.API.Edit(path, old, new, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Edited %s", path)), nil
}

func grep(ctx sdk.Context, args []string) (sdk.Result, error) {
	var pattern, pathPrefix string
	var showLineNums, filesOnly, countOnly bool
	var contextLines int

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n":
			showLineNums = true
		case arg == "-l":
			filesOnly = true
		case arg == "-c":
			countOnly = true
		case arg == "-C" && i+1 < len(args):
			i++
			contextLines, _ = strconv.Atoi(args[i])
		case strings.HasPrefix(arg, "-C"):
			contextLines, _ = strconv.Atoi(arg[2:])
		case !strings.HasPrefix(arg, "-"):
			if pattern == "" {
				pattern = arg
			} else {
				pathPrefix = arg
			}
		}
	}

	if pattern == "" {
		return nil, fmt.Errorf("grep: missing pattern argument")
	}

	results, err := sdk.API.Grep(pattern, sdk.GrepOpts{Path: pathPrefix, Context: contextLines})
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if len(results) == 0 {
		return sdk.Rich{Text: "", Data: []sdk.GrepHit{}}, nil
	}

	var text string
	if countOnly {
		counts := make(map[string]int)
		for _, r := range results {
			counts[r.Path]++
		}
		var out strings.Builder
		for path, count := range counts {
			fmt.Fprintf(&out, "%s:%d\n", path, count)
		}
		text = strings.TrimSuffix(out.String(), "\n")
	} else if filesOnly {
		seen := make(map[string]bool)
		var paths []string
		for _, r := range results {
			if !seen[r.Path] {
				seen[r.Path] = true
				paths = append(paths, r.Path)
			}
		}
		text = strings.Join(paths, "\n")
	} else {
		var out strings.Builder
		for _, r := range results {
			if showLineNums {
				fmt.Fprintf(&out, "%s:%d:%s\n", r.Path, r.Line, r.Text)
			} else {
				fmt.Fprintf(&out, "%s:%s\n", r.Path, r.Text)
			}
		}
		text = strings.TrimSuffix(out.String(), "\n")
	}

	return sdk.Rich{Text: text, Data: results}, nil
}

func glob(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("glob: missing pattern argument")
	}

	paths, err := sdk.API.Glob(args[0])
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	if len(paths) == 0 {
		return sdk.Rich{Text: "", Data: []string{}}, nil
	}

	return sdk.Rich{Text: strings.Join(paths, "\n"), Data: paths}, nil
}

func historyCmd(ctx sdk.Context, args []string) (sdk.Result, error) {
	var path string
	var limit int

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-n" && i+1 < len(args) {
			i++
			limit, _ = strconv.Atoi(args[i])
		} else if strings.HasPrefix(arg, "-n") {
			limit, _ = strconv.Atoi(arg[2:])
		} else if !strings.HasPrefix(arg, "-") {
			path = arg
		}
	}

	if path == "" {
		return nil, fmt.Errorf("history: missing path argument")
	}

	versions, err := sdk.API.History(path, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}

	if len(versions) == 0 {
		return sdk.Rich{Text: "No history found", Data: []sdk.Version{}}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-8s %-12s %-20s %s\n", "Version", "Author", "Date", "Message"))
	sb.WriteString(strings.Repeat("-", 60) + "\n")

	for _, v := range versions {
		date := time.UnixMilli(v.CreatedAt).Format("2006-01-02 15:04:05")
		msg := v.Message
		if msg == "" {
			msg = "-"
		}
		author := v.Author
		if len(author) > 12 {
			author = author[:11] + "…"
		}
		sb.WriteString(fmt.Sprintf("%-8d %-12s %-20s %s\n", v.Num, author, date, msg))
	}

	return sdk.Rich{Text: strings.TrimSuffix(sb.String(), "\n"), Data: versions}, nil
}

func diffCmd(ctx sdk.Context, args []string) (sdk.Result, error) {
	var paths []string
	var contextLines int
	var statOnly bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-C" && i+1 < len(args):
			i++
			contextLines, _ = strconv.Atoi(args[i])
		case arg == "--stat":
			statOnly = true
		case len(arg) > 0 && arg[0] != '-':
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("diff: missing source argument")
	}

	var source, target string
	switch len(paths) {
	case 1:
		versions, err := sdk.API.History(paths[0], 2)
		if err != nil || len(versions) < 2 {
			return nil, fmt.Errorf("diff: no previous version for %s", paths[0])
		}
		source = paths[0] + ":" + strconv.Itoa(versions[1].Num)
		target = paths[0]
	case 2:
		source, target = paths[0], paths[1]
	default:
		return nil, fmt.Errorf("diff: expected 1 or 2 paths, got %d", len(paths))
	}

	diffText, added, removed, err := sdk.API.Diff(source, target, contextLines)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	if statOnly {
		return sdk.Text(fmt.Sprintf("+%d -%d", added, removed)), nil
	}

	if diffText == "" {
		return sdk.Text("No differences"), nil
	}

	return sdk.Text(diffText), nil
}

func restore(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("restore: missing path argument")
	}

	if err := sdk.API.Restore(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Restored %s", args[0])), nil
}

func revert(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("revert: requires <path> <version> arguments")
	}

	path := args[0]
	versionStr := strings.TrimPrefix(args[1], "v")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("revert: invalid version: %s", args[1])
	}

	var message string
	for i := 2; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if strings.HasPrefix(args[i], "--message=") {
			message = strings.TrimPrefix(args[i], "--message=")
		}
	}

	if message == "" {
		message = fmt.Sprintf("Reverted to version %d", version)
	}

	if err := sdk.API.Revert(path, version, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("revert: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Reverted %s to version %d", path, version)), nil
}
