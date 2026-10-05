package filesystem

import (
	"regexp"
	"strings"

	go_pkg_parser "github.com/pardnchiu/go-pkg/filesystem/parser"
)

var paragraphRe = regexp.MustCompile(`\r?\n[\t ]*(?:\r?\n)+`)

func splitParagraphs(source, text string) []go_pkg_parser.Chunk {
	var list []string
	for _, p := range paragraphRe.Split(text, -1) {
		if p = strings.TrimSpace(p); p != "" {
			list = append(list, p)
		}
	}

	chunks := make([]go_pkg_parser.Chunk, len(list))
	for i, p := range list {
		chunks[i] = go_pkg_parser.Chunk{
			Source:  source,
			Index:   i + 1,
			Total:   len(list),
			Content: p,
		}
	}
	return chunks
}
