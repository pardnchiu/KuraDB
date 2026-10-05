package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	go_sqlkit "github.com/pardnchiu/go-sqlkit"
	go_sqlkit_core "github.com/pardnchiu/go-sqlkit/core"
)

//go:embed schema/file_data.sql
var sqlSchemaFileData string

const readPoolSize = 8

type DB struct {
	conn  *go_sqlkit_core.Connector
	Read  *sql.DB
	Write *sql.DB
}

func OpenPerDB(ctx context.Context, path string) (*DB, error) {
	return openWithSchemas(ctx, path, []string{sqlSchemaFileData})
}

func openWithSchemas(ctx context.Context, path string, schemas []string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database: path is required")
	}

	conn, err := go_sqlkit.New(go_sqlkit_core.Config{
		Target:       go_sqlkit_core.SQLite,
		Path:         path,
		MaxOpenConns: readPoolSize,
		MaxIdleConns: readPoolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	db := &DB{conn: conn, Read: conn.Read, Write: conn.Write}
	for i, s := range schemas {
		if _, err := db.Write.ExecContext(ctx, s); err != nil {
			db.Close()
			return nil, fmt.Errorf("database: migrate[%d]: %w", i, err)
		}
	}
	return db, nil
}

func (db *DB) Close() {
	if db == nil || db.conn == nil {
		return
	}
	db.conn.Close()
}
