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

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_keychain "github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/agenvoy/kuradb/internal/database"
	databaseHandler "github.com/agenvoy/kuradb/internal/database/handler"
	"github.com/agenvoy/kuradb/internal/filesystem"
	"github.com/agenvoy/kuradb/internal/openai"
	"github.com/agenvoy/kuradb/internal/runtime"
	"github.com/agenvoy/kuradb/internal/segmenter"
	"github.com/agenvoy/kuradb/internal/vector"
)

const (
	pollInterval  = 10 * time.Second
	embedInterval = 5 * time.Second
	embedBatch    = 64
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "--daemon":
			runServerDaemon()
			return
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
		case "stop":
			cmdStop()
			return
		case "port":
			cmdPort(os.Args[2:])
			return
		case "remote":
			cmdRemote(os.Args[2:])
			return
		case "mcp":
			cmdMCP()
			return
		case "help", "-h", "--help":
			printUsage(os.Stdout)
			return
		}
	}
	runServer()
}

func runServer() {
	// Daemonize: fork into background.
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "os.Executable: %v\n", err)
		os.Exit(1)
	}

	_, configDir := mustConfigDir()

	logFile, err := os.OpenFile(filepath.Join(configDir, "daemon.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open daemon.log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open /dev/null: %v\n", err)
		os.Exit(1)
	}
	defer devNull.Close()

	proc, err := os.StartProcess(exe, []string{exe, "--daemon"}, &os.ProcAttr{
		Files: []*os.File{devNull, logFile, logFile},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "os.StartProcess: %v\n", err)
		os.Exit(1)
	}
	if err := proc.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "proc.Release: %v\n", err)
		os.Exit(1)
	}

	// Wait for the daemon to become ready.
	endpointPath := filepath.Join(configDir, "endpoint")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(endpointPath); err == nil && len(data) > 0 {
			fmt.Println(strings.TrimSpace(string(data)))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "daemon did not become ready within 10s; check %s\n", filepath.Join(configDir, "daemon.log"))
	os.Exit(1)
}

func runServerDaemon() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if err := ctx.Err(); err != nil {
		slog.Info("shutdown before start", "reason", err)
		os.Exit(1)
	}
	defer cancel()

	homeDir, configDir := mustConfigDir()

	// Write runtime.uid so parent daemon spawner can detect readiness.
	if _, err := runtime.Init(configDir); err != nil {
		slog.Warn("runtime.Init", slog.String("error", err.Error()))
	}

	go_pkg_keychain.Init("kuradb", configDir)

	reg := database.New(filepath.Join(configDir, "db.json"))

	embedder, err := openai.New()
	if err != nil {
		slog.Error("openai.New",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	globalDB, err := database.OpenGlobal(ctx, filepath.Join(configDir, "global.db"))
	if err != nil {
		slog.Error("database.OpenGlobal",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer globalDB.Close()

	qcache := openai.NewCache()
	loadQueryCache(ctx, globalDB, qcache)
	qcache.OnSet(func(q string, v []float32) {
		go func() {
			saveCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
			defer c()
			if err := databaseHandler.SaveQueryCache(globalDB, saveCtx, q, openai.Encode(v)); err != nil {
				slog.Warn("query_cache: save",
					slog.String("query", q),
					slog.String("error", err.Error()))
			}
		}()
	})

	segmenter.New()
	vector.New()

	perDBs := make(map[string]*database.DB)

	entries, err := reg.Load()
	if err != nil {
		slog.Error("registry.Load",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	for _, entry := range entries {
		baseDir := filepath.Join(configDir, entry.DB)
		if err := go_pkg_filesystem.CheckDir(baseDir, true); err != nil {
			slog.Warn("db: CheckDir base",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
			continue
		}

		folderDir := filepath.Join(baseDir, "inbox")
		if err := go_pkg_filesystem.CheckDir(folderDir, true); err != nil {
			slog.Warn("db: CheckDir inbox",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
			continue
		}

		linkPath := filepath.Join(homeDir, "Kura_"+entry.DB)
		if err := ensureSymlink(folderDir, linkPath); err != nil {
			slog.Warn("db: ensureSymlink",
				slog.String("db", entry.DB),
				slog.String("link", linkPath),
				slog.String("error", err.Error()))
			continue
		}

		db, err := database.OpenPerDB(ctx, filepath.Join(baseDir, "data.db"))
		if err != nil {
			slog.Warn("db: Open",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
			continue
		}
		perDBs[entry.DB] = db

		if err := vector.InitBucket(entry.DB); err != nil {
			slog.Warn("vector.EnsureBucket",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
			continue
		}
		if err := loadCache(ctx, entry.DB, db); err != nil {
			slog.Warn("loadCache",
				slog.String("db", entry.DB),
				slog.String("error", err.Error()))
		}

		recordPath := filepath.Join(baseDir, "record.json")

		go runEmbedder(ctx, entry.DB, db, embedder, embedInterval, embedBatch)
		go runWatcher(ctx, folderDir, recordPath, db)

		slog.Info("db: ready",
			slog.String("db", entry.DB))
	}

	go runHTTP(ctx, configDir, reg, perDBs, embedder, qcache)

	<-ctx.Done()
	slog.Info("shutdown",
		slog.String("reason", ctx.Err().Error()))

	if err := runtime.Clear(configDir); err != nil {
		slog.Warn("runtime.Clear", slog.String("error", err.Error()))
	}

	for _, db := range perDBs {
		db.Close()
	}
}

func runWatcher(ctx context.Context, folderDir, recordPath string, db *database.DB) {
	var prev *map[string]filesystem.File
	if snap, err := go_pkg_filesystem.ReadJSON[map[string]filesystem.File](recordPath); err == nil {
		prev = &snap
	} else if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("go_pkg_filesystem.ReadJSON",
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
				if err := go_pkg_filesystem.WriteJSON(recordPath, *snap, false); err != nil {
					slog.Warn("go_pkg_filesystem.WriteJSON",
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
	configDir = filepath.Join(homeDir, ".config", "kuradb")
	if err := go_pkg_filesystem.CheckDir(configDir, true); err != nil {
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
