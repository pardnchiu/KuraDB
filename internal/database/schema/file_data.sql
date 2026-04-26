CREATE TABLE IF NOT EXISTS file_data (
    id      INTEGER PRIMARY KEY,
    source  TEXT    NOT NULL,
    chunk   INTEGER NOT NULL,
    total   INTEGER NOT NULL,
    content TEXT    NOT NULL,
    dismiss BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (source, chunk)
);
CREATE INDEX IF NOT EXISTS idx_file_data_source ON file_data(source);
