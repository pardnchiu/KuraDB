package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pardnchiu/AgenvoyRAG/internal/filesystem/parser"
)

//go:embed schema/file_data.sql
var sqlSchemaFileData string

type DB struct {
	db *sql.DB
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

	s := &DB{db: raw}
	if err := s.migrate(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (db *DB) Close() {
	if db == nil || db.db == nil {
		return
	}
	db.db.Close()
}

func (db *DB) migrate(ctx context.Context) error {
	if _, err := db.db.ExecContext(ctx, sqlSchemaFileData); err != nil {
		return fmt.Errorf("database: migrate: %w", err)
	}
	return nil
}

func (db *DB) Save(ctx context.Context, source string, files []parser.FileData) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("database: not initialized")
	}
	if source == "" {
		return fmt.Errorf("database: source is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: db.db.BeginTx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE file_data
SET dismiss = TRUE
WHERE source = ?
AND dismiss = FALSE;
`, source); err != nil {
		return fmt.Errorf("database: tx.ExecContext: %w", err)
	}

	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
INSERT INTO file_data (source, chunk, total, content, dismiss)
VALUES (?, ?, ?, ?, FALSE)
ON CONFLICT (source, chunk)
DO UPDATE SET
    total   = excluded.total,
    content = excluded.content,
    dismiss = FALSE;`,
			f.Source, f.Index, f.Total, f.Content,
		); err != nil {
			return fmt.Errorf("database: tx.ExecContext (chunk=%d): %w", f.Index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("database: tx.Commit: %w", err)
	}
	return nil
}
