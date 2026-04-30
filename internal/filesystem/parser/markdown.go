package parser

import (
	"context"
	"fmt"
	"os"
)

func Markdown(ctx context.Context, path string) ([]FileData, error) {
	if path == "" {
		return nil, fmt.Errorf("markdown: path is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("markdown: os.ReadFile: %w", err)
	}

	docs := splitParagraphs(ctx, path, string(b))
	if len(docs) == 0 {
		return nil, fmt.Errorf("markdown: %q empty", path)
	}
	return docs, nil
}
