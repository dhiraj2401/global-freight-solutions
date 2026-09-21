// Package gfs embeds the site's templates, static assets and migrations and
// exposes the HTTP handler shared by the local server (cmd/server) and the
// Vercel function (api/index.go).
//
// Embedding keeps deployment to a single self-contained binary: nothing is
// read from disk at runtime, which is what serverless platforms require.
package gfs

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/gloitel/global-freight-solutions/internal/config"
	"github.com/gloitel/global-freight-solutions/internal/server"
)

//go:embed templates static
var webFS embed.FS

//go:embed db/migrations/*.sql
var migrationsFS embed.FS

// Assets returns the embedded templates/ and static/ trees.
func Assets() fs.FS { return webFS }

// Migrations returns the embedded db/migrations tree.
func Migrations() fs.FS { return migrationsFS }

// NewLogger returns a JSON logger for production and a text logger otherwise.
func NewLogger(production bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if os.Getenv("LOG_LEVEL") == "debug" {
		opts.Level = slog.LevelDebug
	}
	if production {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

// NewHandler builds the application handler and a cleanup function.
func NewHandler(ctx context.Context, cfg config.Config, logger *slog.Logger) (http.Handler, func(), error) {
	return server.New(ctx, cfg, webFS, logger)
}

var (
	serverlessOnce    sync.Once
	serverlessHandler http.Handler
	serverlessErr     error
)

// ServeServerless lazily initialises the handler once per function instance
// (reused across warm invocations, including the database pool) and serves r.
func ServeServerless(w http.ResponseWriter, r *http.Request) {
	serverlessOnce.Do(func() {
		cfg, err := config.Load()
		if err != nil {
			serverlessErr = err
			return
		}
		logger := NewLogger(cfg.IsProduction())
		slog.SetDefault(logger)
		serverlessHandler, _, serverlessErr = NewHandler(context.Background(), cfg, logger)
	})
	if serverlessErr != nil {
		slog.Error("startup failed", "error", serverlessErr)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	serverlessHandler.ServeHTTP(w, r)
}
