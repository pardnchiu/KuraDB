package databaseHandler

import (
	"context"
	"fmt"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
)

func LoadEmbeding(db *database.DB, ctx context.Context, fn func(id int64, blob []byte) error) error {
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
SELECT id, embedding
FROM file_data
WHERE is_embed = TRUE
AND dismiss = FALSE
AND embedding IS NOT NULL;
`)
	if err != nil {
		return fmt.Errorf("db.DB.QueryContext: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return fmt.Errorf("rows.Scan: %w", err)
		}
		if err := fn(id, blob); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows.Err: %w", err)
	}
	return nil
}
