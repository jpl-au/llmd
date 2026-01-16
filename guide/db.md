# llmd db

List databases or manage their local/shared status.

## Usage

```bash
llmd db                    # list all databases
llmd db notes              # show status of llmd-notes.db
llmd db --local            # mark default database as local
llmd db notes --local      # mark notes database as local
llmd db notes --share      # mark as shared
llmd db --dir /path        # list databases in external directory
```

## Flags

| Flag | Description |
|------|-------------|
| `-l, --local` | Mark database as local |
| `-s, --share` | Mark database as shared |
| `--dir` | Target directory (default: discover from current directory) |

## Output

```bash
$ llmd db
llmd.db        shared
llmd-notes.db  local
llmd-docs.db   shared
```

## Examples

```bash
# Create a local database for personal notes
llmd init --db notes --local

# Later, decide to share it
llmd db notes --share

# Or mark an existing shared database as local
llmd db docs --local

# Mark default database as local (no name needed)
llmd db --local

# Manage databases in an external project
llmd db --dir /path/to/other/project
llmd db --dir /path/to/other/project notes --local
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `LLMD_DB` | Default database name (equivalent to `--db`) |
| `LLMD_DIR` | Default database directory (equivalent to `--dir`) |

```bash
LLMD_DB=docs llmd ls           # use llmd-docs.db
LLMD_DIR=/path/to/.llmd llmd ls  # use database in another directory
```

## Notes

- Local databases are added to `.llmd/.gitignore`
- Shared databases are committed to the repository
- Use `llmd init --db name --local` to create a local database
- If no name is given with `--local` or `--share`, operates on the default database
- Use `--dir` to manage databases in external projects
