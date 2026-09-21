package gfs_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gfs "github.com/gloitel/global-freight-solutions"
	"github.com/gloitel/global-freight-solutions/internal/config"
)

// TestHandlerRoutes exercises the fully wired handler with embedded assets,
// as it runs on Vercel and in the container.
func TestHandlerRoutes(t *testing.T) {
	cfg, err := config.LoadFrom(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	handler, cleanup, err := gfs.NewHandler(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tests := []struct {
		method, path string
		status       int
		contains     string
	}{
		{http.MethodGet, "/", http.StatusOK, "Moving Businesses Further."},
		{http.MethodHead, "/", http.StatusOK, ""},
		{http.MethodGet, "/static/css/app.css", http.StatusOK, ""},
		{http.MethodGet, "/static/js/htmx.min.js", http.StatusOK, ""},
		{http.MethodGet, "/static/img/hero-port.jpg", http.StatusOK, ""},
		{http.MethodGet, "/static/", http.StatusNotFound, ""},
		{http.MethodGet, "/healthz", http.StatusOK, `"ok"`},
		{http.MethodGet, "/robots.txt", http.StatusOK, "User-agent"},
		{http.MethodGet, "/api/quote", http.StatusMethodNotAllowed, ""},
		{http.MethodGet, "/does-not-exist", http.StatusNotFound, "wrong turn"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if tt.contains != "" && !strings.Contains(rec.Body.String(), tt.contains) {
				t.Errorf("body missing %q", tt.contains)
			}
			if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
				t.Errorf("CSP = %q", csp)
			}
		})
	}
}
