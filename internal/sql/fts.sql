CREATE VIRTUAL TABLE IF NOT EXISTS content_fts USING fts5(
    key,
    path,
    content,
    content=content,
    content_rowid=id
);

CREATE TRIGGER IF NOT EXISTS content_fts_insert AFTER INSERT ON content BEGIN
    INSERT INTO content_fts(rowid, key, path, content)
    VALUES (new.id, new.key, new.path, new.content);
END;

CREATE TRIGGER IF NOT EXISTS content_fts_delete AFTER DELETE ON content BEGIN
    INSERT INTO content_fts(content_fts, rowid, key, path, content)
    VALUES ('delete', old.id, old.key, old.path, old.content);
END;
