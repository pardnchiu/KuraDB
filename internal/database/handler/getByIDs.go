package databaseHandler

import (
	"context"
	"fmt"
	"strings"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
)

type FileRow struct {
	ID      int64
	Source  string
	Chunk   int
	Total   int
	Content string
	Rank    float64
}

func GetByIDs(db *database.DB, ctx context.Context, ids []int64) ([]FileRow, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("db is required")
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	query := fmt.Sprintf(`
SELECT id, source, chunk, total, content
FROM file_data
WHERE id IN (%s)
AND dismiss = FALSE;
`, placeholders)

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db.DB.QueryContext: %w", err)
	}
	defer rows.Close()

	results := make([]FileRow, 0, len(ids))
	for rows.Next() {
		var r FileRow
		if err := rows.Scan(&r.ID, &r.Source, &r.Chunk, &r.Total, &r.Content); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows.Err: %w", err)
	}
	return results, nil
}
