CREATE VIRTUAL TABLE IF NOT EXISTS content_fts USING fts5(
    key,
    path,
    content,
    content=content,
    content_rowid=id
);
