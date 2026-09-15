package databaseHandler

import (
	"context"
	"fmt"

	"github.com/pardnchiu/kuradb/internal/database"
)

func Dismiss(db *database.DB, ctx context.Context, source string) error {
	if db == nil || db.Write == nil {
		return fmt.Errorf("db is required")
	}
	if source == "" {
		return fmt.Errorf("source is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if _, err := db.Write.ExecContext(ctx, `
UPDATE file_data
SET dismiss = TRUE,
    updated_at = CURRENT_TIMESTAMP
WHERE source = ?
AND dismiss = FALSE;`, source); err != nil {
		return fmt.Errorf("db.db.ExecContext: %w", err)
	}
	return nil
}
