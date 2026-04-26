package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	goUtils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
	"github.com/pardnchiu/AgenvoyRAG/internal/filesystem"
)

const (
	binaryName   = "AgenRAG"
	pollInterval = 10 * time.Second
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if err := ctx.Err(); err != nil {
		slog.Info("shutdown before start", "reason", err)
		os.Exit(1)
	}
	defer cancel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("os.UserHomeDir",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	if homeDir == "" {
		slog.Error("home directory is empty")
		os.Exit(1)
	}

	folderDir := filepath.Join(homeDir, binaryName)
	if err := goUtils_filesystem.CheckDir(folderDir, true); err != nil {
		slog.Error("goUtils_filesystem.CheckDir",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	dbDir := filepath.Join(homeDir, ".config", "Agenvoy", "rag")
	if err := goUtils_filesystem.CheckDir(dbDir, true); err != nil {
		slog.Error("goUtils_filesystem.CheckDir",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	st, err := database.Open(ctx, filepath.Join(dbDir, "data.db"))
	if err != nil {
		slog.Error("database.Open",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer st.Close()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var prev *map[string]filesystem.File

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown",
				slog.String("reason", ctx.Err().Error()))
			return

		case <-ticker.C:
			prev = filesystem.WalkFiles(ctx, folderDir, folderDir, prev, st)
		}
	}
}
