package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Home renders the landing page.
func (app *App) Home(w http.ResponseWriter, r *http.Request) {
	page := app.newPage()
	page.Preload = app.site.Hero.Image.Src
	page.CompactForm = app.newQuoteForm(variantCompact, quoteInput{}, nil, "")
	page.FullForm = app.newQuoteForm(variantFull, quoteInput{}, nil, "")
	// The page is identical for every visitor, so let the CDN cache it briefly.
	w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=300, stale-while-revalidate=86400")
	app.renderPage(w, http.StatusOK, "index", page)
}

// NotFound renders the 404 page.
func (app *App) NotFound(w http.ResponseWriter, _ *http.Request) {
	app.renderPage(w, http.StatusNotFound, "result", app.newPage())
}

// Health reports liveness. With ?db=1 it also checks database connectivity.
func (app *App) Health(w http.ResponseWriter, r *http.Request) {
	status, body := http.StatusOK, map[string]string{"status": "ok"}
	if r.URL.Query().Get("db") == "1" {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := app.db.Ping(ctx); err != nil {
			app.logger.Error("health: database ping failed", "error", err)
			status, body = http.StatusServiceUnavailable, map[string]string{"status": "degraded", "database": "unreachable"}
		} else {
			body["database"] = "ok"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Robots serves robots.txt.
func (app *App) Robots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /api/\n"))
}
