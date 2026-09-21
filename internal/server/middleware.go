package server

import (
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gloitel/global-freight-solutions/internal/config"
)

func withRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.Error("panic serving request", "path", r.URL.Path, "panic", rec, "stack", string(debug.Stack()))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withSecurityHeaders sets a strict CSP. All scripts and styles are served
// from this origin (htmx included) apart from Google Fonts; no inline script
// or style is used anywhere, so 'unsafe-inline' is never needed.
func withSecurityHeaders(cfg config.Config, next http.Handler) http.Handler {
	formAction := "'self'"
	if cfg.TrackingURL != "" {
		if u, err := url.Parse(cfg.TrackingURL); err == nil {
			// Chrome applies form-action to redirects after a form POST.
			formAction += " " + u.Scheme + "://" + u.Host
		}
	}
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
		"img-src 'self' data:",
		"connect-src 'self'",
		"form-action " + formAction,
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"object-src 'none'",
	}, "; ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		if cfg.IsProduction() {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// withRequestLog logs method, path, status and latency. It deliberately never
// logs query strings, form values or client IPs.
func withRequestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		level := slog.LevelInfo
		if strings.HasPrefix(r.URL.Path, "/static/") {
			level = slog.LevelDebug
		}
		logger.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"htmx", r.Header.Get("HX-Request") == "true",
		)
	})
}
