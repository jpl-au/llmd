CREATE TABLE IF NOT EXISTS entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    namespace TEXT NOT NULL,
    path TEXT NOT NULL,
    value TEXT NOT NULL,
    author TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_entities_key ON entities(key);
CREATE INDEX IF NOT EXISTS idx_entities_ns_path ON entities(namespace, path);
CREATE INDEX IF NOT EXISTS idx_entities_ns_path_created ON entities(namespace, path, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_entities_deleted ON entities(deleted_at) WHERE deleted_at IS NOT NULL;
