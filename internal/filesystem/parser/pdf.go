package parser

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type FileData struct {
	Source  string
	Index   int
	Total   int
	Content string
}

func PDF(ctx context.Context, path string) ([]FileData, error) {
	if path == "" {
		return nil, fmt.Errorf("pdf: path is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdf: pdftotext: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	pages := strings.Split(stdout.String(), "\f")
	if n := len(pages); n > 0 && pages[n-1] == "" {
		pages = pages[:n-1]
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("pdf: %q empty", path)
	}

	total := len(pages)
	docs := make([]FileData, 0, total)
	for i, content := range pages {
		if err := ctx.Err(); err != nil {
			return docs, err
		}
		docs = append(docs, FileData{
			Source:  path,
			Index:   i + 1,
			Total:   total,
			Content: content,
		})
	}
	return docs, nil
}
