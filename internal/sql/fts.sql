CREATE VIRTUAL TABLE IF NOT EXISTS content_fts USING fts5(
    key,
    path,
    content,
    content=content,
    content_rowid=id
);

-- Remove legacy triggers (FTS index is maintained by the event bus)
DROP TRIGGER IF EXISTS content_fts_insert;
DROP TRIGGER IF EXISTS content_fts_delete;
