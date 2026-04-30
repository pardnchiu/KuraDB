package databaseHandler

import (
	"context"
	"fmt"

	"github.com/pardnchiu/KuraDB/internal/database"
)

func SaveQueryCache(db *database.DB, ctx context.Context, query string, blob []byte) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("db is required")
	}
	if query == "" {
		return fmt.Errorf("query is required")
	}
	if len(blob) == 0 {
		return fmt.Errorf("blob is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if _, err := db.DB.ExecContext(ctx, `
INSERT INTO query_cache (query, embedding)
VALUES (?, ?)
ON CONFLICT (query) DO UPDATE SET
  embedding  = excluded.embedding,
  created_at = CURRENT_TIMESTAMP;
`, query, blob); err != nil {
		return fmt.Errorf("db.DB.ExecContext: %w", err)
	}
	return nil
}

func LoadQueryCache(db *database.DB, ctx context.Context, fn func(query string, blob []byte) error) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("db is required")
	}
	if fn == nil {
		return fmt.Errorf("fn is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	rows, err := db.DB.QueryContext(ctx, `
SELECT query, embedding
FROM query_cache;
`)
	if err != nil {
		return fmt.Errorf("db.DB.QueryContext: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var q string
		var blob []byte
		if err := rows.Scan(&q, &blob); err != nil {
			return fmt.Errorf("rows.Scan: %w", err)
		}
		if err := fn(q, blob); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows.Err: %w", err)
	}
	return nil
}
