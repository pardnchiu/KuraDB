package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pardnchiu/kuradb/internal/database"
	"github.com/pardnchiu/kuradb/internal/mcp"
	"github.com/pardnchiu/kuradb/internal/segmenter"
)

func cmdMCP() {
	_, configDir := mustConfigDir()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	reg := database.New(filepath.Join(configDir, "db.json"))

	segmenter.New()

	entries, err := reg.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp: registry.Load: %v\n", err)
		os.Exit(1)
	}

	dbs := make(map[string]*database.DB, len(entries))
	defer func() {
		for _, db := range dbs {
			db.Close()
		}
	}()

	for _, entry := range entries {
		db, err := database.OpenPerDB(ctx, filepath.Join(configDir, entry.DB, "data.db"))
		if err != nil {
			slog.Warn("mcp: database.OpenPerDB",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
			continue
		}
		dbs[entry.DB] = db
	}

	if err := mcp.Run(ctx, reg, dbs); err != nil && !isSessionEnd(err) {
		fmt.Fprintf(os.Stderr, "mcp: %v\n", err)
		os.Exit(1)
	}
}

func isSessionEnd(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF) ||
		strings.HasSuffix(err.Error(), io.EOF.Error())
}
