-- 004_links.sql: Document relationships for knowledge graphs.
--
-- Links represent edges between documents, enabling relationship discovery
-- and navigation. Both endpoints are tracked with their source tables to
-- support cross-table linking in extensions.

CREATE TABLE IF NOT EXISTS links (
    id TEXT PRIMARY KEY,                        -- 8-char unique identifier
    from_path TEXT NOT NULL,                    -- Source document path
    from_source TEXT NOT NULL DEFAULT 'documents',  -- Source table
    to_path TEXT NOT NULL,                      -- Target document path
    to_source TEXT NOT NULL DEFAULT 'documents',    -- Target table
    tag TEXT NOT NULL DEFAULT '',               -- Optional link type label
    created_at INTEGER NOT NULL,                -- Unix timestamp of creation
    deleted_at INTEGER,                         -- Unix timestamp of soft delete, NULL if active
    UNIQUE(from_path, from_source, to_path, to_source, tag)
);

CREATE INDEX IF NOT EXISTS idx_links_from ON links(from_path);
CREATE INDEX IF NOT EXISTS idx_links_to ON links(to_path);
CREATE INDEX IF NOT EXISTS idx_links_tag ON links(tag);
CREATE INDEX IF NOT EXISTS idx_links_from_source ON links(from_source);
CREATE INDEX IF NOT EXISTS idx_links_to_source ON links(to_source);
