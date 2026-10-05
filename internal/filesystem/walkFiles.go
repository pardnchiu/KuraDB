package filesystem

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_pkg_parser "github.com/pardnchiu/go-pkg/filesystem/parser"

	"github.com/pardnchiu/kuradb/internal/database"
	databaseHandler "github.com/pardnchiu/kuradb/internal/database/handler"
)

type File struct {
	Size     int64
	ModTime  time.Time
	IsDir    bool
	Children *map[string]File
}

func WalkFiles(ctx context.Context, root, dir string, prev *map[string]File, db *database.DB) *map[string]File {
	if err := ctx.Err(); err != nil {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Warn("os.ReadDir",
			slog.String("error", err.Error()))
		return nil
	}

	result := make(map[string]File, len(entries))
	present := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		present[e.Name()] = struct{}{}
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return &result
		}

		path := filepath.Join(dir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			slog.Warn("entry.Info",
				slog.String("error", err.Error()))
			continue
		}

		data := File{
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
		}

		unchanged := false
		var prevChildren *map[string]File
		if prev != nil {
			if p, ok := (*prev)[entry.Name()]; ok && p.IsDir == data.IsDir {
				prevChildren = p.Children
				if p.Size == data.Size && p.ModTime.Equal(data.ModTime) {
					unchanged = true
				}
			}
		}

		if !unchanged {
			slog.Info("changed",
				slog.String("path", path))

			if !data.IsDir {
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				kind := "text"
				var (
					files []go_pkg_parser.Chunk
					text  string
					err   error
					skip  bool
				)
				switch {
				case shouldSkipByName(entry.Name()), shouldSkipByExt(ext):
					skip = true
				case ext == ".pdf":
					kind = "pdf"
					text, _, err = go_pkg_parser.PDF(ctx, path)
					text = strings.ReplaceAll(text, "\f", "")
				case ext == ".docx":
					kind = "docx"
					text, _, err = go_pkg_parser.Docx(ctx, path)
				case ext == ".pptx":
					kind = "pptx"
					_, files, err = go_pkg_parser.PPTX(ctx, path)
				case ext == ".csv", ext == ".tsv":
					kind = "csv"
					files, err = parseTable(ctx, path, go_pkg_parser.CSV)
				case ext == ".xlsx":
					kind = "xlsx"
					files, err = parseTable(ctx, path, go_pkg_parser.XLSX)
				default:
					if !looksLikeText(path) {
						skip = true
						break
					}
					text, _, err = go_pkg_parser.Markdown(ctx, path)
				}
				if text != "" {
					files = splitParagraphs(path, text)
				}
				if !skip {
					if err != nil {
						slog.Warn("parser",
							slog.String("error", err.Error()))
					} else if perr := databaseHandler.Upsert(db, ctx, path, files); perr != nil {
						slog.Warn("store.Save",
							slog.String("error", perr.Error()))
					} else {
						slog.Info("saved",
							slog.String("kind", kind),
							slog.String("path", path),
							slog.Int("chunks", len(files)))
					}
				}
			}
		}

		if entry.IsDir() {
			data.Children = WalkFiles(ctx, root, path, prevChildren, db)
		}

		result[entry.Name()] = data
	}

	if prev != nil && ctx.Err() == nil {
		for name, p := range *prev {
			if _, ok := present[name]; ok {
				continue
			}
			dismissRemoved(ctx, filepath.Join(dir, name), p, db)
		}
	}

	return &result
}

func dismissRemoved(ctx context.Context, path string, node File, db *database.DB) {
	if err := ctx.Err(); err != nil {
		return
	}
	if node.IsDir {
		if node.Children == nil {
			return
		}
		for childName, childNode := range *node.Children {
			dismissRemoved(ctx, filepath.Join(path, childName), childNode, db)
		}
		return
	}
	if err := databaseHandler.Dismiss(db, ctx, path); err != nil {
		slog.Warn("db.Dismiss",
			slog.String("path", path),
			slog.String("error", err.Error()))
		return
	}
	slog.Info("dismissed",
		slog.String("path", path))
}
