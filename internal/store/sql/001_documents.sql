-- 001_documents.sql: Core document storage table and indexes.
--
-- Documents are versioned - each write creates a new row with incremented version.
-- Soft-delete via deleted_at allows recovery until vacuum permanently removes data.

CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,  -- Internal row ID
    key TEXT NOT NULL,                     -- 8-char unique identifier for external reference
    path TEXT NOT NULL,                    -- Virtual path (e.g., "docs/readme")
    content TEXT NOT NULL,                 -- Document content (markdown)
    version INTEGER NOT NULL DEFAULT 1,    -- Version number, increments on each write
    author TEXT NOT NULL,                  -- Who created this version
    message TEXT,                          -- Optional commit message
    created_at INTEGER NOT NULL,           -- Unix timestamp of creation
    deleted_at INTEGER,                    -- Unix timestamp of soft delete, NULL if active
    UNIQUE(path, version)
);

CREATE INDEX IF NOT EXISTS idx_documents_key ON documents(key);
CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path);
CREATE INDEX IF NOT EXISTS idx_documents_path_version ON documents(path, version DESC);
CREATE INDEX IF NOT EXISTS idx_documents_deleted ON documents(deleted_at);
CREATE INDEX IF NOT EXISTS idx_documents_path_deleted ON documents(path, deleted_at);
