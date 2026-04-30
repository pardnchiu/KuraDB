package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema/file_data.sql
var sqlSchemaFileData string

//go:embed schema/query_cache.sql
var sqlSchemaQueryCache string

type DB struct {
	DB *sql.DB
}

func Open(ctx context.Context, path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database: path is required")
	}

	dsn := fmt.Sprintf(
		"file:%s?_journal_mode=WAL&_busy_timeout=15000&_synchronous=NORMAL&_foreign_keys=on",
		path,
	)

	raw, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1)

	if err := raw.PingContext(ctx); err != nil {
		raw.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	db := &DB{DB: raw}
	if err := db.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() {
	if db == nil || db.DB == nil {
		return
	}
	db.DB.Close()
}

func (db *DB) migrate(ctx context.Context) error {
	if _, err := db.DB.ExecContext(ctx, sqlSchemaFileData); err != nil {
		return fmt.Errorf("database: migrate file_data: %w", err)
	}
	if _, err := db.DB.ExecContext(ctx, sqlSchemaQueryCache); err != nil {
		return fmt.Errorf("database: migrate query_cache: %w", err)
	}
	return nil
}
