# llmd init

Create a new store in the current directory.

## Usage

```
llmd init
```

## Rules

- Creates `.llmd/llmd.db` in the current directory, plus a
  `.llmd/.gitignore` that commits the database and ignores its
  temporary files. Prints `Initialised llmd store at <path>`.
- The global `--db <path>` flag creates the database at a different
  path instead.
- Safe to run where a store already exists: it does not overwrite
  existing data.
- Needs no author. Over MCP the tool is named `llmd_init`; `init` is
  not available over HTTP.

## Examples

```bash
# Initialise in the current directory
llmd init

# Initialise at a custom path
llmd --db /path/to/store.db init
```

## Notes

After `init`, set your author with `llmd config author "Your Name"`
and read `llmd guide`. Agents do not set the configured author; they
pass `--author` on every call instead. See `guide config`.
