-- 003_tags.sql: Document tagging for organization and filtering.
--
-- Tags are metadata about documents, stored separately so they persist across
-- document versions. Tagging "docs/readme" applies to all versions of that path.

CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,                        -- 8-char unique identifier
    path TEXT NOT NULL,                         -- Document path
    source TEXT NOT NULL DEFAULT 'documents',   -- Source table
    tag TEXT NOT NULL,                          -- Tag label
    created_at INTEGER NOT NULL,                -- Unix timestamp of creation
    deleted_at INTEGER,                         -- Unix timestamp of soft delete, NULL if active
    UNIQUE(path, source, tag)
);

CREATE INDEX IF NOT EXISTS idx_tags_path ON tags(path);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
CREATE INDEX IF NOT EXISTS idx_tags_source ON tags(source);
