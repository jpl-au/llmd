# llmd init

Initialise a new llmd store in the current directory.

## Usage

```bash
llmd init                        # initialise in current directory
llmd init --db docs              # create additional database (llmd-docs.db)
llmd init --dir /path/to/project # initialise in external directory
llmd init --local                # mark database as local (gitignored)
llmd init --force                # reinitialise (destructive)
```

## Flags

| Flag | Description |
|------|-------------|
| `--db` | Database name (creates llmd-{name}.db) |
| `--dir` | Target directory (default: current directory) |
| `-l, --local` | Mark database as local (not committed) |
| `--force` | Reinitialise, removing existing database |

## What it creates

```
.llmd/
  llmd.db       # SQLite database (commit this)
  .gitignore    # Excludes *.md and config.yaml
```

Note: `init` does not create config. Use `llmd config` to set up author and other settings. This follows the git model where init only creates the repository structure.

## Examples

```bash
# Basic init
llmd init

# Create database in external directory
llmd init --dir /path/to/other/project

# Reinitialise (deletes existing data!)
llmd init --force
```

## Multiple Databases

A project can have multiple independent databases:

```bash
llmd init                       # creates llmd.db (default)
llmd init --db docs             # creates llmd-docs.db
llmd init --db notes --local    # creates llmd-notes.db, not committed
```

Select a database with `--db` flag or `LLMD_DB` env var:

```bash
llmd --db docs ls              # list docs in llmd-docs.db
LLMD_DB=docs llmd ls           # same thing
LLMD_DIR=/other/project llmd ls  # use database in another directory
```

Use `llmd db` to list databases and manage local/shared status.

## Flag Combinations

- `--dir` and `--local` cannot be used together
  - `--local` modifies the current project's .gitignore
  - `--dir` creates the database elsewhere
  - To mark an external database as local, use `llmd db --dir /path --local`

## Notes

- Run this once per project, in the project root
- Database files (`*.db`) should be committed (they're the source of truth)
- Mirrored `*.md` files and `config.yaml` are gitignored
- Set author with `llmd config author.name "Your Name"`
