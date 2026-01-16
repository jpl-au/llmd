-- 002_documents_fts.sql: Full-text search using SQLite FTS5.
--
-- FTS5 Trigger Design Decision:
-- These triggers index all documents including soft-deleted ones. This is intentional.
-- The search query filters by deleted_at at query time, but keeping deleted docs in the
-- index enables the -D (deleted only) and -A (include all) search flags to work.
-- Conditional triggers that skip soft-deleted docs would break these features.
-- The minor index bloat is cleaned up when vacuum hard-deletes old documents.

CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    path,
    content,
    content=documents,
    content_rowid=id
);

CREATE TRIGGER IF NOT EXISTS documents_fts_insert AFTER INSERT ON documents BEGIN
    INSERT INTO documents_fts(rowid, path, content)
    VALUES (new.id, new.path, new.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_fts_delete AFTER DELETE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, path, content)
    VALUES('delete', old.id, old.path, old.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_fts_update AFTER UPDATE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, path, content)
    VALUES('delete', old.id, old.path, old.content);
    INSERT INTO documents_fts(rowid, path, content)
    VALUES (new.id, new.path, new.content);
END;
