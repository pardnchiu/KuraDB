package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	go_pkg_parser "github.com/pardnchiu/go-pkg/filesystem/parser"
)

const rowsPerChunk = 32

type tableParserFn func(ctx context.Context, path string, offset, limit int) (string, error)

func parseTable(ctx context.Context, path string, fn tableParserFn) ([]go_pkg_parser.Chunk, error) {
	raw, err := fn(ctx, path, 1, math.MaxInt32)
	if err != nil {
		return nil, err
	}

	var rows [][]string
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, nil
	}
	if len(rows) < 2 {
		return nil, nil
	}

	header := rows[0]
	data := rows[1:]
	total := (len(data) + rowsPerChunk - 1) / rowsPerChunk

	chunks := make([]go_pkg_parser.Chunk, 0, total)
	for i := 0; i < len(data); i += rowsPerChunk {
		if err := ctx.Err(); err != nil {
			return chunks, err
		}
		end := min(i+rowsPerChunk, len(data))

		var b strings.Builder
		for j, row := range data[i:end] {
			if j > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "[%d] ", i+j+1)
			for k, cell := range row {
				if k >= len(header) {
					break
				}
				if k > 0 {
					b.WriteString(", ")
				}
				key := header[k]
				if key == "" {
					key = fmt.Sprintf("col%d", k+1)
				}
				b.WriteString(key)
				b.WriteByte('=')
				b.WriteString(cell)
			}
		}

		chunks = append(chunks, go_pkg_parser.Chunk{
			Source:  path,
			Index:   len(chunks) + 1,
			Total:   total,
			Content: b.String(),
		})
	}
	return chunks, nil
}
