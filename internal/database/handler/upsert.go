package databaseHandler

import (
	"context"
	"fmt"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
	"github.com/pardnchiu/AgenvoyRAG/internal/filesystem/parser"
)

func Upsert(db *database.DB, ctx context.Context, source string, files []parser.FileData) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("db is required")
	}
	if source == "" {
		return fmt.Errorf("source is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.DB.BeginTx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE file_data
SET dismiss = TRUE,
  updated_at = CURRENT_TIMESTAMP
WHERE source = ?
AND dismiss = FALSE;
`, source); err != nil {
		return fmt.Errorf("tx.ExecContext: %w", err)
	}

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
INSERT INTO file_data (source, chunk, total, content, dismiss)
VALUES (?, ?, ?, ?, FALSE)
ON CONFLICT (source, chunk)
DO UPDATE SET
  total      = excluded.total,
  content    = excluded.content,
  dismiss    = FALSE,
  embedding  = CASE WHEN file_data.content = excluded.content
                    THEN file_data.embedding ELSE NULL  END,
  is_embed   = CASE WHEN file_data.content = excluded.content
                    THEN file_data.is_embed  ELSE FALSE END,
  updated_at = CURRENT_TIMESTAMP;`,
			file.Source, file.Index, file.Total, file.Content,
		); err != nil {
			return fmt.Errorf("tx.ExecContext (chunk=%d): %w", file.Index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("tx.Commit: %w", err)
	}
	return nil
}
