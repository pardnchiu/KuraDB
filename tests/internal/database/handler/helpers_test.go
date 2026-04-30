package databaseHandler_integration_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pardnchiu/KuraDB/internal/database"
)

func openPerDB(t *testing.T) *database.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.OpenPerDB(context.Background(), filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("OpenPerDB: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func openGlobalDB(t *testing.T) *database.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.OpenGlobal(context.Background(), filepath.Join(dir, "global.db"))
	if err != nil {
		t.Fatalf("OpenGlobal: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func seedFileData(t *testing.T, db *database.DB, source string, total int, contents []string) []int64 {
	t.Helper()
	if len(contents) != total {
		t.Fatalf("seedFileData: contents len %d != total %d", len(contents), total)
	}
	ids := make([]int64, 0, total)
	for i, c := range contents {
		res, err := db.DB.ExecContext(context.Background(),
			`INSERT INTO file_data (source, chunk, total, content) VALUES (?, ?, ?, ?)`,
			source, i+1, total, c,
		)
		if err != nil {
			t.Fatalf("seed insert: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("LastInsertId: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
