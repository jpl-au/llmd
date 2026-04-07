CREATE TABLE IF NOT EXISTS entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL,
    namespace TEXT NOT NULL,
    relation TEXT,
    value TEXT NOT NULL,
    author TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_entities_key ON entities(key);
CREATE INDEX IF NOT EXISTS idx_entities_ns_relation ON entities(namespace, relation);
CREATE INDEX IF NOT EXISTS idx_entities_ns_relation_created ON entities(namespace, relation, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_entities_deleted ON entities(deleted_at) WHERE deleted_at IS NOT NULL;
