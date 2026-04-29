package parser

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

func DOCX(ctx context.Context, path string) ([]FileData, error) {
	if path == "" {
		return nil, fmt.Errorf("docx: path is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("docx: zip.OpenReader: %w", err)
	}
	defer zr.Close()

	var inner *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			inner = f
			break
		}
	}
	if inner == nil {
		return nil, fmt.Errorf("docx: word/document.xml missing")
	}

	rc, err := inner.Open()
	if err != nil {
		return nil, fmt.Errorf("docx: inner Open: %w", err)
	}
	defer rc.Close()

	text, err := docxText(ctx, rc)
	if err != nil {
		return nil, fmt.Errorf("docx: parse: %w", err)
	}

	docs := splitParagraphs(ctx, path, text)
	if len(docs) == 0 {
		return nil, fmt.Errorf("docx: %q empty", path)
	}
	return docs, nil
}

func docxText(ctx context.Context, r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var b strings.Builder
	tableDepth := 0
	pendingNewline := false
	pendingTab := false

	flush := func() {
		if pendingNewline {
			b.WriteByte('\n')
			pendingNewline = false
		}
		if pendingTab {
			b.WriteByte('\t')
			pendingTab = false
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return b.String(), err
		}
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return b.String(), err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tbl":
				tableDepth++
			case "t":
				var content string
				if err := dec.DecodeElement(&content, &t); err != nil {
					return b.String(), err
				}
				flush()
				b.WriteString(content)
			case "tab":
				flush()
				b.WriteByte('\t')
			case "br":
				flush()
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if tableDepth > 0 {
					pendingNewline = true
				} else {
					b.WriteString("\n\n")
				}
			case "tc":
				if tableDepth > 0 {
					pendingNewline = false
					pendingTab = true
				}
			case "tr":
				if tableDepth > 0 {
					pendingTab = false
					b.WriteByte('\n')
				}
			case "tbl":
				if tableDepth > 0 {
					tableDepth--
				}
				if tableDepth == 0 {
					pendingNewline = false
					pendingTab = false
					b.WriteString("\n\n")
				}
			}
		}
	}
	return b.String(), nil
}
