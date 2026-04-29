package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/pardnchiu/AgenvoyRAG/internal/api"
	"github.com/pardnchiu/AgenvoyRAG/internal/database"
	"github.com/pardnchiu/AgenvoyRAG/internal/openai"
	"github.com/pardnchiu/AgenvoyRAG/internal/segmenter"
	"github.com/pardnchiu/AgenvoyRAG/internal/vector"
)

const (
	httpHost      = "127.0.0.1"
	httpPortMin   = 10000
	httpPortMax   = 65535
	httpBindTries = 10

	httpReadHeaderTimeout = 5 * time.Second
	httpShutdownTimeout   = 5 * time.Second
)

func runHTTP(
	ctx context.Context,
	db *database.DB,
	cache *vector.Cache,
	embedder openai.Embedder,
	qcache *openai.Cache,
	seg *segmenter.Segmenter,
) {
	ln, port, err := pickListener()
	if err != nil {
		slog.Error("http: pickListener",
			slog.String("error", err.Error()))
		return
	}

	srv := &http.Server{
		Handler:           api.Router(db, cache, embedder, qcache, seg),
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Warn("http: Shutdown",
				slog.String("error", err.Error()))
		}
	}()

	url := fmt.Sprintf("http://%s:%d", httpHost, port)
	slog.Info("http server",
		slog.String("url", url))

	if err := openBrowser(url + "/api/health"); err != nil {
		slog.Warn("http: openBrowser",
			slog.String("error", err.Error()))
	}

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http: Serve",
			slog.String("error", err.Error()))
	}
}

func pickListener() (net.Listener, int, error) {
	for range httpBindTries {
		port := httpPortMin + rand.IntN(httpPortMax-httpPortMin+1)
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", httpHost, port))
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no free port in [%d, %d] after %d tries", httpPortMin, httpPortMax, httpBindTries)
}

func openBrowser(url string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return fmt.Errorf("openBrowser: unsupported OS: %s", runtime.GOOS)
	}
	return exec.Command(cmd, url).Start()
}
