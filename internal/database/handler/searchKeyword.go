package databaseHandler

import (
	"context"
	"fmt"
	"strings"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
)

func SearchKeyword(db *database.DB, ctx context.Context, keywords []string, limit int) ([]FileRow, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("db is required")
	}
	if len(keywords) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	caseClauses := make([]string, len(keywords))
	likeClauses := make([]string, len(keywords))
	for i := range keywords {
		caseClauses[i] = "CASE WHEN LOWER(content) LIKE ? THEN 1 ELSE 0 END"
		likeClauses[i] = "LOWER(content) LIKE ?"
	}

	var sb strings.Builder
	sb.WriteString(`
SELECT id, source, chunk, total, content, (`)
	sb.WriteString(strings.Join(caseClauses, " + "))
	sb.WriteString(`) AS hits
FROM file_data
WHERE dismiss = FALSE
AND (`)
	sb.WriteString(strings.Join(likeClauses, " OR "))
	sb.WriteString(`)
ORDER BY hits DESC, id ASC
LIMIT ?;`)

	args := make([]any, 0, len(keywords)*2+1)
	for _, kw := range keywords {
		args = append(args, "%"+kw+"%")
	}
	for _, kw := range keywords {
		args = append(args, "%"+kw+"%")
	}
	args = append(args, limit)

	rows, err := db.DB.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("db.DB.QueryContext: %w", err)
	}
	defer rows.Close()

	out := make([]FileRow, 0)
	for rows.Next() {
		var r FileRow
		if err := rows.Scan(&r.ID, &r.Source, &r.Chunk, &r.Total, &r.Content, &r.Rank); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows.Err: %w", err)
	}
	return out, nil
}
