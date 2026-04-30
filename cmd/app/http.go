package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"time"

	"github.com/pardnchiu/KuraDB/internal/agenvoy"
	"github.com/pardnchiu/KuraDB/internal/api"
	"github.com/pardnchiu/KuraDB/internal/database"
	"github.com/pardnchiu/KuraDB/internal/openai"
	"github.com/pardnchiu/KuraDB/internal/segmenter"
	"github.com/pardnchiu/KuraDB/internal/vector"
)

const (
	httpPortMin   = 10000
	httpPortMax   = 65535
	httpBindTries = 10

	httpReadHeaderTimeout = 5 * time.Second
	httpShutdownTimeout   = 5 * time.Second
)

func runHTTP(ctx context.Context, dbName string, db *database.DB, cache *vector.Cache, embedder openai.Embedder, qcache *openai.Cache, seg *segmenter.Segmenter) {
	ln, port, err := pickListener()
	if err != nil {
		slog.Error("http: pickListener",
			slog.String("error", err.Error()))
		return
	}

	url := fmt.Sprintf("http://%s:%d", "127.0.0.1", port)

	if err := agenvoy.Register(dbName, url); err != nil {
		slog.Warn("agenvoy.Register",
			slog.String("error", err.Error()))
	}

	srv := &http.Server{
		Handler:           api.Router(dbName, db, cache, embedder, qcache, seg),
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}

	go func() {
		<-ctx.Done()
		if err := agenvoy.Unregister(); err != nil {
			slog.Warn("agenvoy.Unregister",
				slog.String("error", err.Error()))
		}
		shutCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Warn("http: Shutdown",
				slog.String("error", err.Error()))
		}
	}()

	slog.Info("http server",
		slog.String("url", url))

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http: Serve",
			slog.String("error", err.Error()))
	}
}

func pickListener() (net.Listener, int, error) {
	for range httpBindTries {
		port := httpPortMin + rand.IntN(httpPortMax-httpPortMin+1)
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "127.0.0.1", port))
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no free port in [%d, %d] after %d tries", httpPortMin, httpPortMax, httpBindTries)
}
