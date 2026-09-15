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

	go_pkg_keychain "github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/kuradb/internal/database"
	"github.com/pardnchiu/kuradb/internal/mcp"
	"github.com/pardnchiu/kuradb/internal/openai"
	"github.com/pardnchiu/kuradb/internal/segmenter"
	"github.com/pardnchiu/kuradb/internal/vector"
)

func cmdMCP() {
	_, configDir := mustConfigDir()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go_pkg_keychain.Init("kuradb", configDir)

	reg := database.New(filepath.Join(configDir, "db.json"))

	embedder, err := openai.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp: openai.New: %v\n", err)
		os.Exit(1)
	}

	globalDB, err := database.OpenGlobal(ctx, filepath.Join(configDir, "global.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp: database.OpenGlobal: %v\n", err)
		os.Exit(1)
	}
	defer globalDB.Close()

	qcache := openai.NewCache()
	loadQueryCache(ctx, globalDB, qcache)

	segmenter.New()
	vector.New()

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

		if err := vector.InitBucket(entry.DB); err != nil {
			slog.Warn("mcp: vector.InitBucket",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
			continue
		}
		if err := loadCache(ctx, entry.DB, db); err != nil {
			slog.Warn("mcp: loadCache",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
		}
	}

	if err := mcp.Run(ctx, reg, dbs, embedder, qcache); err != nil && !isSessionEnd(err) {
		fmt.Fprintf(os.Stderr, "mcp: %v\n", err)
		os.Exit(1)
	}
}

func isSessionEnd(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF) ||
		strings.HasSuffix(err.Error(), io.EOF.Error())
}
