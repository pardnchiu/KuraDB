CREATE TABLE IF NOT EXISTS query_cache (
    query      TEXT      PRIMARY KEY,
    embedding  BLOB      NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
