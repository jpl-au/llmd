# llmd init

Initialise a new llmd document store.

## Usage

```
llmd init
```

## Examples

```bash
# Initialise in the current directory
llmd init

# Initialise at a custom path
llmd --db /path/to/store.db init
```

## Notes

- Creates `.llmd/llmd.db` in the current directory by default.
- Use the global `--db` flag to create the database at a different path.
- Safe to run in a directory that already has a store; it will not overwrite existing data.
