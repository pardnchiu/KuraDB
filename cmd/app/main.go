package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	goUtils_filesystem "github.com/pardnchiu/go-utils/filesystem"
	goUtils_utils "github.com/pardnchiu/go-utils/utils"

	"github.com/pardnchiu/KuraDB/internal/database"
	"github.com/pardnchiu/KuraDB/internal/filesystem"
	"github.com/pardnchiu/KuraDB/internal/openai"
	"github.com/pardnchiu/KuraDB/internal/segmenter"
	"github.com/pardnchiu/KuraDB/internal/vector"
)

const (
	pollInterval  = 10 * time.Second
	embedInterval = 5 * time.Second
	embedBatch    = 64
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "add":
			cmdAdd(os.Args[2:])
			return
		case "list":
			cmdList(os.Args[2:])
			return
		case "remove":
			cmdRemove(os.Args[2:])
			return
		case "edit":
			cmdEdit(os.Args[2:])
			return
		case "help", "-h", "--help":
			printUsage(os.Stdout)
			return
		}
	}
	runServer()
}

func runServer() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if err := ctx.Err(); err != nil {
		slog.Info("shutdown before start", "reason", err)
		os.Exit(1)
	}
	defer cancel()

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Error("godotenv.Load",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	dbName := sanitizeDBName(goUtils_utils.GetWithDefault("DB_NAME", ""))
	if dbName == "" {
		slog.Error("DB_NAME is required (set in .env or environment)")
		os.Exit(1)
	}

	homeDir, configDir := mustConfigDir()

	baseDir := filepath.Join(configDir, dbName)
	if err := goUtils_filesystem.CheckDir(baseDir, true); err != nil {
		slog.Error("goUtils_filesystem.CheckDir",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	reg := database.New(filepath.Join(configDir, "db.json"))
	if err := reg.AddIfMissing(dbName); err != nil {
		slog.Warn("registry.AddIfMissing",
			slog.String("error", err.Error()))
	}

	folderDir := filepath.Join(baseDir, "inbox")
	if err := goUtils_filesystem.CheckDir(folderDir, true); err != nil {
		slog.Error("goUtils_filesystem.CheckDir",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	linkPath := filepath.Join(homeDir, "Kura_"+dbName)
	if err := ensureSymlink(folderDir, linkPath); err != nil {
		slog.Error("ensureSymlink",
			slog.String("link", linkPath),
			slog.String("target", folderDir),
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	db, err := database.Open(ctx, filepath.Join(baseDir, "data.db"))
	if err != nil {
		slog.Error("database.Open",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	embedder, err := openai.New()
	if err != nil {
		slog.Error("openai.New",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	seg, err := segmenter.New()
	if err != nil {
		slog.Error("segmenter.New",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	cache := vector.New()
	if err := loadCache(ctx, db, cache); err != nil {
		slog.Warn("loadCache",
			slog.String("error", err.Error()))
	}

	qcache := openai.NewCache()

	recordPath := filepath.Join(baseDir, "record.json")

	go runEmbedder(ctx, db, embedder, embedInterval, embedBatch)
	go runHTTP(ctx, dbName, db, cache, embedder, qcache, seg)
	go runWatcher(ctx, folderDir, recordPath, db)

	<-ctx.Done()
	slog.Info("shutdown",
		slog.String("reason", ctx.Err().Error()))
}

func runWatcher(ctx context.Context, folderDir, recordPath string, db *database.DB) {
	var prev *map[string]filesystem.File
	if snap, err := goUtils_filesystem.ReadJSON[map[string]filesystem.File](recordPath); err == nil {
		prev = &snap
	} else if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("goUtils_filesystem.ReadJSON",
			slog.String("error", err.Error()))
	}

	var saveMu sync.Mutex

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("watcher: shutdown",
				slog.String("reason", ctx.Err().Error()))
			return

		case <-ticker.C:
			prev = filesystem.WalkFiles(ctx, folderDir, folderDir, prev, db)
			if prev == nil {
				continue
			}
			snap := prev
			go func() {
				saveMu.Lock()
				defer saveMu.Unlock()
				if err := goUtils_filesystem.WriteJSON(recordPath, *snap, false); err != nil {
					slog.Warn("goUtils_filesystem.WriteJSON",
						slog.String("error", err.Error()))
				}
			}()
		}
	}
}

func sanitizeDBName(s string) string {
	return strings.Join(strings.Fields(s), "_")
}

func mustConfigDir() (homeDir, configDir string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "os.UserHomeDir: %v\n", err)
		os.Exit(1)
	}
	if homeDir == "" {
		fmt.Fprintln(os.Stderr, "home directory is empty")
		os.Exit(1)
	}
	configDir = filepath.Join(homeDir, ".config", "KuraDB")
	if err := goUtils_filesystem.CheckDir(configDir, true); err != nil {
		fmt.Fprintf(os.Stderr, "CheckDir %s: %v\n", configDir, err)
		os.Exit(1)
	}
	return homeDir, configDir
}

func ensureSymlink(target, link string) error {
	info, err := os.Lstat(link)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("lstat %s: %w", link, err)
		}
		if err := os.Symlink(target, link); err != nil {
			return fmt.Errorf("symlink %s -> %s: %w", link, target, err)
		}
		return nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("path exists and is not a symlink: %s", link)
	}
	current, err := os.Readlink(link)
	if err != nil {
		return fmt.Errorf("readlink %s: %w", link, err)
	}
	if current == target {
		return nil
	}
	if err := os.Remove(link); err != nil {
		return fmt.Errorf("remove stale symlink %s: %w", link, err)
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", link, target, err)
	}
	return nil
}
