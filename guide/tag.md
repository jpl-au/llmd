# llmd tag

Manage document tags for semantic categorisation.

## Usage

```bash
llmd tag add <path|key> <tag>
llmd tag rm <path|key> <tag>
llmd tag ls [path|key]
```

The `add` and `rm` subcommands accept either a document path or an 8-character key.

## Description

Tags allow you to categorise documents beyond the folder structure. A document can have multiple tags, and you can filter lists by tag.

## Examples

### Add Tags

```bash
llmd tag add docs/api "needs-review"
llmd tag add docs/api "v1"
```

### Remove Tags

```bash
llmd tag rm docs/api "needs-review"
```

### List Tags

```bash
llmd tag ls docs/api      # List tags for a document
llmd tag ls               # List all tags in the store
```

### Filter by Tag

Use the `--tag` flag with `llmd ls` to find documents with a specific tag:

```bash
llmd ls --tag "needs-review"
```
