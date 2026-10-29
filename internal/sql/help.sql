-- Help table for LLMs and humans exploring the database directly.
-- One row, replaced when the binary's help text changes.
CREATE TABLE IF NOT EXISTS help (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    content TEXT
);
