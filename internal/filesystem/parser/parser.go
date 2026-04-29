package parser

import (
	"context"
	"regexp"
	"strings"
)

var paragraphRe = regexp.MustCompile(`\n[\t ]*\n+`)

func splitParagraphs(ctx context.Context, source, text string) []FileData {
	parts := paragraphRe.Split(text, -1)
	nonEmpty := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			nonEmpty = append(nonEmpty, t)
		}
	}
	if len(nonEmpty) == 0 {
		return nil
	}

	total := len(nonEmpty)
	docs := make([]FileData, 0, total)
	for i, p := range nonEmpty {
		if err := ctx.Err(); err != nil {
			return docs
		}
		docs = append(docs, FileData{
			Source:  source,
			Index:   i + 1,
			Total:   total,
			Content: p,
		})
	}
	return docs
}
