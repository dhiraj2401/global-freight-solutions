// Package handlers implements the HTTP endpoints. Every response is HTML:
// full pages for navigation, fragments for htmx requests.
package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gloitel/global-freight-solutions/internal/config"
	"github.com/gloitel/global-freight-solutions/internal/content"
	"github.com/gloitel/global-freight-solutions/internal/db"
	"github.com/gloitel/global-freight-solutions/internal/email"
	"github.com/gloitel/global-freight-solutions/internal/templates"
)

// App holds handler dependencies.
type App struct {
	cfg           config.Config
	db            db.Store
	emailProvider email.EmailProvider
	templates     *templates.Renderer
	site          *content.Site
	logger        *slog.Logger
	now           func() time.Time
}

// Deps are the collaborators required by App.
type Deps struct {
	Config   config.Config
	Store    db.Store
	Email    email.EmailProvider
	Renderer *templates.Renderer
	Site     *content.Site
	Logger   *slog.Logger
}

// New constructs an App.
func New(d Deps) *App {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		cfg:           d.Config,
		db:            d.Store,
		emailProvider: d.Email,
		templates:     d.Renderer,
		site:          d.Site,
		logger:        logger,
		now:           time.Now,
	}
}

// pageData is the root value for full-page templates.
type pageData struct {
	Site                  *content.Site
	ShowPlaceholderBanner bool
	Preload               string
	CompactForm           quoteFormView
	FullForm              quoteFormView
	// Result and ResultData drive templates/result.html.
	Result     string
	ResultData any
}

func (app *App) newPage() pageData {
	return pageData{
		Site:                  app.site,
		ShowPlaceholderBanner: app.site.HasPlaceholders && !app.cfg.IsProduction(),
	}
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func (app *App) renderPage(w http.ResponseWriter, status int, page string, data pageData) {
	var buf bytes.Buffer
	if err := app.templates.Page(&buf, page, data); err != nil {
		app.logger.Error("render page failed", "page", page, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// renderFragment sends partial to htmx requests. Regular form posts (no
// JavaScript) get the same fragment wrapped in a full page.
func (app *App) renderFragment(w http.ResponseWriter, r *http.Request, status int, partial, resultKind string, data any) {
	w.Header().Add("Vary", "HX-Request")
	w.Header().Set("Cache-Control", "no-store")
	if !isHTMX(r) {
		page := app.newPage()
		page.Result = resultKind
		page.ResultData = data
		app.renderPage(w, status, "result", page)
		return
	}
	var buf bytes.Buffer
	if err := app.templates.Partial(&buf, partial, data); err != nil {
		app.logger.Error("render partial failed", "partial", partial, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// sameOrigin is the CSRF check for form posts. Browsers always send Origin on
// cross-site POSTs, and scripts cannot forge it. A request is accepted when
// Origin matches the host it was sent to (covers preview deployments) or an
// entry in ALLOWED_ORIGIN. Requests without Origin are accepted only when
// Sec-Fetch-Site doesn't indicate a cross-site request (older browsers, curl).
func (app *App) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		switch r.Header.Get("Sec-Fetch-Site") {
		case "", "same-origin", "none":
			return true
		default:
			return false
		}
	}
	if origin == "null" {
		return false
	}
	for _, allowed := range app.cfg.AllowedOrigins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && strings.EqualFold(u.Host, r.Host)
}

// clientIP returns the caller's IP. Proxy headers are only trusted when the
// platform guarantees them (Vercel overwrites X-Forwarded-For / X-Real-IP).
func (app *App) clientIP(r *http.Request) string {
	if app.cfg.TrustProxyHeaders {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// hashIP pseudonymises an IP with a keyed hash so raw addresses are never
// stored but repeat submissions can still be rate limited.
func hashIP(salt, ip string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}
