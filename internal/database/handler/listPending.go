package databaseHandler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pardnchiu/kuradb/internal/database"
)

type Pending struct {
	ID      int64
	Source  string
	Content string
}

func ListPending(db *database.DB, ctx context.Context, limit int) ([]Pending, error) {
	if db == nil || db.Read == nil {
		return nil, fmt.Errorf("db is required")
	}
	if limit <= 0 {
		slog.Warn("ListPending: invalid limit, using 10 as default")
		limit = 10
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := db.Read.QueryContext(ctx, `
SELECT id, source, content
FROM file_data
WHERE is_embed = FALSE
AND dismiss = FALSE
ORDER BY id ASC
LIMIT ?;
`, limit)
	if err != nil {
		return nil, fmt.Errorf("db.Read.QueryContext: %w", err)
	}
	defer rows.Close()

	results := make([]Pending, 0)
	for rows.Next() {
		var p Pending
		if err := rows.Scan(&p.ID, &p.Source, &p.Content); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows.Err: %w", err)
	}
	return results, nil
}
