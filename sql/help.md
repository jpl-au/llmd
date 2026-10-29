# llmd SQL Reference

llmd stores markdown documents in SQLite. Two tables matter for reading:

- `documents` - versioned document storage
- `fts` - full-text search index (FTS5)

---

## documents table

Columns: `path`, `content`, `version`, `author`, `message`, `created_at`, `deleted_at`

Each row is a version. Same path appears multiple times with incrementing version.

### Get a document's content

```sql
SELECT content
FROM documents
WHERE path = 'notes/todo'
  AND deleted_at IS NULL
ORDER BY version DESC
LIMIT 1
```

We filter out deleted documents, sort by version descending to get the newest first, and limit to 1.

### See a document's history

```sql
SELECT version, author, message, created_at
FROM documents
WHERE path = 'notes/todo'
ORDER BY version
```

### List all documents

Returns the latest version of each document:

```sql
SELECT path, version, author, created_at
FROM documents
WHERE deleted_at IS NULL
GROUP BY path
HAVING version = MAX(version)
```

The `HAVING version = MAX(version)` keeps only the highest version per path.

### Get content from a specific version

```sql
SELECT content
FROM documents
WHERE path = 'notes/todo'
  AND version = 3
```

---

## fts table (Full-Text Search)

Columns: `path`, `content`

The `fts` table is an FTS5 virtual table that indexes the latest version of each document.

### Basic search

```sql
SELECT path FROM fts WHERE content MATCH 'sqlite'
```

### Search with snippets

```sql
SELECT path, snippet(fts, 1, '[', ']', '...', 20) as preview
FROM fts
WHERE content MATCH 'sqlite'
```

The `snippet()` function extracts text around matches. Arguments: `snippet(table, column, before, after, ellipsis, max_tokens)`:
- `1` - column index (0=path, 1=content)
- `'['` / `']'` - markers wrapped around matched terms, so "sqlite" becomes "[sqlite]"
- `'...'` - shown when text is truncated
- `20` - maximum words to return

### Search with relevance ranking

```sql
SELECT path, rank
FROM fts
WHERE content MATCH 'sqlite database'
ORDER BY rank
```

Lower rank = better match.

### Phrase search

```sql
SELECT path FROM fts WHERE content MATCH '"version control"'
```

Quotes around the phrase require the words to appear consecutively.

### Boolean operators

```sql
SELECT path FROM fts WHERE content MATCH 'sqlite AND markdown'
SELECT path FROM fts WHERE content MATCH 'sqlite OR postgres'
SELECT path FROM fts WHERE content MATCH 'database NOT mysql'
```

### Prefix search

```sql
SELECT path FROM fts WHERE content MATCH 'data*'
```

Matches "data", "database", "datatype", etc.
