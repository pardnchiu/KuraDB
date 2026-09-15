package databaseHandler

import (
	"context"
	"fmt"

	"github.com/pardnchiu/kuradb/internal/database"
)

type EmbeddingItem struct {
	ID        int64
	Content   string
	Embedding []byte
}

func UpdateEmbedding(db *database.DB, ctx context.Context, items []EmbeddingItem) ([]int64, error) {
	if db == nil || db.Write == nil {
		return nil, fmt.Errorf("db is required")
	}
	if len(items) == 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tx, err := db.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("db.Write.BeginTx: %w", err)
	}
	defer tx.Rollback()

	applied := make([]int64, 0, len(items))
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res, err := tx.ExecContext(ctx, `
UPDATE file_data
SET embedding  = ?,
  is_embed   = TRUE,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
AND dismiss = FALSE
AND content = ?;
`, item.Embedding, item.ID, item.Content)
		if err != nil {
			return nil, fmt.Errorf("tx.ExecContext (id=%d): %w", item.ID, err)
		}

		num, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("res.RowsAffected (id=%d): %w", item.ID, err)
		}
		if num > 0 {
			applied = append(applied, item.ID)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("tx.Commit: %w", err)
	}
	return applied, nil
}
