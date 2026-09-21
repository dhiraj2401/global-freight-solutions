// Package server wires configuration, storage, email, templates and handlers
// into a single http.Handler used by both the local server and Vercel.
package server

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gloitel/global-freight-solutions/internal/config"
	"github.com/gloitel/global-freight-solutions/internal/content"
	"github.com/gloitel/global-freight-solutions/internal/db"
	"github.com/gloitel/global-freight-solutions/internal/email"
	"github.com/gloitel/global-freight-solutions/internal/handlers"
	"github.com/gloitel/global-freight-solutions/internal/templates"
)

// New builds the application handler. assets must contain templates/ and
// static/. The returned cleanup releases the database pool.
func New(ctx context.Context, cfg config.Config, assets fs.FS, logger *slog.Logger) (http.Handler, func(), error) {
	site := content.Default()
	resolveImages(site, assets, logger)
	if site.HasPlaceholders && cfg.IsProduction() {
		logger.Warn("site content still contains PLACEHOLDER values; review internal/content/site.go before launch")
	}

	renderer, err := templates.New(assets, templates.Funcs(assets))
	if err != nil {
		return nil, nil, err
	}

	provider, err := email.NewEmailProvider(ctx, cfg.EmailProvider, email.Options{
		To:       cfg.SalesNotificationEmail,
		FromName: cfg.EmailFromName,
		Lookup:   cfg.Lookup,
	})
	if err != nil {
		return nil, nil, err
	}

	var store db.Store
	if cfg.DatabaseURL != "" {
		pool, err := db.NewPool(ctx, cfg.DatabaseURL, db.PoolOptions{MaxConns: cfg.DBMaxConns})
		if err != nil {
			return nil, nil, err
		}
		store = db.NewPostgresStore(pool)
	} else {
		if cfg.IsProduction() {
			return nil, nil, fmt.Errorf("server: DATABASE_URL is required in production")
		}
		logger.Warn("DATABASE_URL not set: using in-memory store, submissions will not persist")
		store = db.NewMemoryStore()
	}

	app := handlers.New(handlers.Deps{
		Config:   cfg,
		Store:    store,
		Email:    provider,
		Renderer: renderer,
		Site:     site,
		Logger:   logger,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", app.Home)
	mux.HandleFunc("POST /api/quote", app.QuoteHandler)
	mux.HandleFunc("POST /api/track", app.TrackHandler)
	// Without these, the "GET /" catch-all would answer GET with 404. (A
	// method-less "/api/quote" pattern would conflict with "GET /" and panic.)
	mux.HandleFunc("GET /api/quote", methodNotAllowed(http.MethodPost))
	mux.HandleFunc("GET /api/track", methodNotAllowed(http.MethodPost))
	mux.HandleFunc("GET /healthz", app.Health)
	mux.HandleFunc("GET /robots.txt", app.Robots)
	mux.Handle("GET /static/", staticHandler(assets))
	mux.HandleFunc("GET /", app.NotFound)

	logger.Info("server configured",
		"env", cfg.Env,
		"email_provider", provider.Name(),
		"database", map[bool]string{true: "postgres", false: "memory"}[cfg.DatabaseURL != ""],
	)

	handler := withRecovery(logger, withSecurityHeaders(cfg, withRequestLog(logger, mux)))
	return handler, store.Close, nil
}

// resolveImages clears image paths whose files are not embedded so templates
// render the designed fallback instead of a broken image.
func resolveImages(site *content.Site, assets fs.FS, logger *slog.Logger) {
	for _, img := range site.Images() {
		if img.Src == "" || !strings.HasPrefix(img.Src, "/static/") {
			continue
		}
		if _, err := fs.Stat(assets, strings.TrimPrefix(img.Src, "/")); err != nil {
			logger.Debug("image not found, using fallback", "src", img.Src)
			img.Src = ""
		}
	}
}

func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allow)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func staticHandler(assets fs.FS) http.Handler {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err) // embedded layout is fixed at compile time
	}
	files := http.StripPrefix("/static/", http.FileServerFS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r) // no directory listings
			return
		}
		if r.URL.Query().Get("v") != "" {
			// Fingerprinted by the asset template func: safe to cache forever.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
		}
		files.ServeHTTP(w, r)
	})
}
