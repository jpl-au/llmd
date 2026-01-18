# llmd grep

Search documents using regular expressions.

## Usage

```bash
llmd grep <pattern> [path]
llmd grep -i <pattern> [path]
```

## Examples

```bash
# Search all documents
llmd grep "TODO"

# Search under a path
llmd grep "authentication" docs/

# Case insensitive search
llmd grep -i "error" docs/

# Regex alternation
llmd grep "error|warning" docs/

# Match patterns
llmd grep "func.*\(" src/

# Character classes
llmd grep "[0-9]{3}" docs/

# List matching paths only
llmd grep -l "error"

# Invert match (show non-matching lines)
llmd grep -v "TODO" docs/

# Show context around matches
llmd grep -C 2 "error" docs/        # 2 lines before and after

# Count matches per document
llmd grep -c "TODO"

# Search recursively in subdirectories
llmd grep -r "TODO" docs/

# JSON output
llmd grep "TODO" -o json
```

## Flags

| Flag | Description |
|------|-------------|
| `-i, --ignore-case` | Ignore case distinctions |
| `-v, --invert-match` | Select non-matching lines |
| `-c, --count` | Only print count of matches per document |
| `-C, --context` | Print N lines of context around matches |
| `-l, --files-with-matches` | Only output paths of matching files |
| `-r, --recursive` | Search subdirectories recursively |
| `-D, --deleted` | Search deleted documents only |
| `-A, --all` | Search all documents (including deleted) |

See `llmd guide` for global flags.

## Output Format

Output follows the standard grep format:
```
path:line:content
```

For example:
```
docs/api:15:## Error Handling
docs/api:17:The API returns standard HTTP error codes
```

## Notes

- Uses Go regular expression syntax (RE2)
- Case-sensitive by default, use `-i` for case-insensitive
- Path argument scopes search to that prefix
- Without `-r`, only searches direct children
- With `-r`, searches all nested paths recursively
- For full-text search (FTS5), use `llmd find` instead
