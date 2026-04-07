CREATE TABLE IF NOT EXISTS content (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL,
    namespace TEXT NOT NULL,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    version INTEGER NOT NULL,
    hash TEXT NOT NULL,
    author TEXT NOT NULL,
    message TEXT,
    source TEXT NOT NULL,
    mime TEXT,
    meta TEXT,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER,
    UNIQUE(namespace, path, version)
);

CREATE INDEX IF NOT EXISTS idx_content_key ON content(key);
CREATE INDEX IF NOT EXISTS idx_content_ns_path ON content(namespace, path);
CREATE INDEX IF NOT EXISTS idx_content_ns_path_version ON content(namespace, path, version DESC);
CREATE INDEX IF NOT EXISTS idx_content_hash ON content(hash);
CREATE INDEX IF NOT EXISTS idx_content_deleted ON content(deleted_at) WHERE deleted_at IS NOT NULL;
