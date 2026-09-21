package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gloitel/global-freight-solutions/internal/config"
	"github.com/gloitel/global-freight-solutions/internal/content"
	"github.com/gloitel/global-freight-solutions/internal/db"
	"github.com/gloitel/global-freight-solutions/internal/email"
	"github.com/gloitel/global-freight-solutions/internal/templates"
)

type fakeProvider struct {
	mu   sync.Mutex
	sent []email.QuoteNotification
	err  error
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) SendQuoteNotification(_ context.Context, n email.QuoteNotification) email.SendResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, n)
	return email.SendResult{Provider: "fake", MessageID: "msg-1", Err: f.err}
}

type testEnv struct {
	app   *App
	store *db.MemoryStore
	email *fakeProvider
}

func newTestEnv(t *testing.T, mutate func(*config.Config)) testEnv {
	t.Helper()
	cfg, err := config.LoadFrom(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&cfg)
	}
	assets := os.DirFS("../..")
	renderer, err := templates.New(assets, templates.Funcs(assets))
	if err != nil {
		t.Fatal(err)
	}
	store := db.NewMemoryStore()
	provider := &fakeProvider{}
	app := New(Deps{
		Config:   cfg,
		Store:    store,
		Email:    provider,
		Renderer: renderer,
		Site:     content.Default(),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return testEnv{app: app, store: store, email: provider}
}

func validCompact() url.Values {
	return url.Values{
		"variant":      {"compact"},
		"full_name":    {"Jane Doe"},
		"origin":       {"Houston, TX"},
		"destination":  {"Dallas, TX"},
		"service_type": {"ftl"},
		"weight_kg":    {"1200"},
		"email":        {"jane@acme.test"},
		"phone":        {"+1 (713) 555-0100"},
	}
}

func post(handler http.HandlerFunc, path string, form url.Values, htmx bool, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

const sameOrigin = "http://example.com" // httptest requests target example.com

func TestHomeRendersPage(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := httptest.NewRecorder()
	env.app.Home(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	// html/template entity-encodes "+" in attributes; browsers decode it back.
	for _, want := range []string{`id="quote-compact"`, `id="quote-full"`, `hx-post="/api/quote"`, `hx-target="#quote-compact"`, `href="tel:&#43;17135550198"`} {
		if !strings.Contains(body, want) {
			t.Errorf("home page missing %s", want)
		}
	}
	// The track dialog has its own pattern, so match the phone hint instead.
	if strings.Count(body, `type="tel"`) != 2 || strings.Count(body, `title="Enter a phone number with area code`) != 2 {
		t.Error("both phone fields should carry the client-side pattern")
	}
	if strings.Contains(body, "ZgotmplZ") {
		t.Error("html/template rejected a URL or attribute (ZgotmplZ in output)")
	}
}

func TestQuoteSuccess(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := post(env.app.QuoteHandler, "/api/quote", validCompact(), true, sameOrigin)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Your request is in") || !strings.Contains(body, `id="quote-compact"`) {
		t.Errorf("unexpected success fragment: %s", body)
	}
	if strings.Contains(body, "<html") {
		t.Error("htmx request should receive a fragment, not a full page")
	}

	quotes := env.store.Quotes()
	if len(quotes) != 1 {
		t.Fatalf("stored %d quotes, want 1", len(quotes))
	}
	q := quotes[0]
	if q.FirstName != "Jane" || q.LastName != "Doe" || q.ServiceType != "ftl" || q.WeightKg == nil || *q.WeightKg != 1200 {
		t.Errorf("stored quote = %+v", q)
	}
	if q.Phone != "+17135550100" {
		t.Errorf("phone stored as %q, want normalized E.164", q.Phone)
	}
	if !strings.HasPrefix(q.Reference, "GFS-") || !strings.Contains(body, q.Reference) {
		t.Errorf("reference %q not shown in response", q.Reference)
	}
	if q.IPHash == "" || strings.Contains(q.IPHash, "192.0.2") {
		t.Errorf("IP must be stored hashed, got %q", q.IPHash)
	}

	if len(env.email.sent) != 1 || env.email.sent[0].Email != "jane@acme.test" || env.email.sent[0].ServiceType != "Full Truckload (FTL)" {
		t.Errorf("notification = %+v", env.email.sent)
	}
	if events := env.store.Events(); len(events) != 1 || events[0].EventType != "sent" || events[0].ProviderMessageID != "msg-1" {
		t.Errorf("email events = %+v", events)
	}
}

func TestQuoteValidationErrors(t *testing.T) {
	env := newTestEnv(t, nil)
	form := validCompact()
	form.Del("email")
	form.Set("phone", "12")
	form.Set("service_type", "teleportation")
	form.Set("full_name", `<script>alert(1)</script>`)

	rec := post(env.app.QuoteHandler, "/api/quote", form, true, sameOrigin)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Please enter your email address.", "Please enter a complete phone number", "Please choose a service type.", `aria-invalid="true"`} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("user input echoed without escaping")
	}
	if len(env.store.Quotes()) != 0 || len(env.email.sent) != 0 {
		t.Error("invalid submission must not be stored or emailed")
	}
}

func TestQuoteFullVariantValidation(t *testing.T) {
	env := newTestEnv(t, nil)
	form := url.Values{"variant": {"full"}, "email": {"jane@acme.test"}}
	rec := post(env.app.QuoteHandler, "/api/quote", form, true, sameOrigin)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="quote-full"`) || !strings.Contains(body, "Please enter your first name.") {
		t.Errorf("unexpected body: %s", body)
	}
	if !strings.Contains(body, `value="jane@acme.test"`) {
		t.Error("valid values should be preserved when re-rendering")
	}
}

func TestQuoteRejectsCrossOrigin(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := post(env.app.QuoteHandler, "/api/quote", validCompact(), true, "https://evil.test")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(env.store.Quotes()) != 0 {
		t.Error("cross-origin submission was stored")
	}
}

func TestQuoteHoneypot(t *testing.T) {
	env := newTestEnv(t, nil)
	form := validCompact()
	form.Set("website", "https://spam.test")
	rec := post(env.app.QuoteHandler, "/api/quote", form, true, sameOrigin)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(env.store.Quotes()) != 0 || len(env.email.sent) != 0 {
		t.Error("honeypot submission must be discarded silently")
	}
}

func TestQuoteEmailFailureStillSucceeds(t *testing.T) {
	env := newTestEnv(t, nil)
	env.email.err = errors.New("smtp: 421 try later")
	rec := post(env.app.QuoteHandler, "/api/quote", validCompact(), true, sameOrigin)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(env.store.Quotes()) != 1 {
		t.Fatal("lead must be saved even when email fails")
	}
	events := env.store.Events()
	if len(events) != 1 || events[0].EventType != "failed" || !strings.Contains(events[0].ErrorMessage, "421") {
		t.Errorf("email events = %+v", events)
	}
}

func TestQuoteRateLimit(t *testing.T) {
	env := newTestEnv(t, func(c *config.Config) { c.QuoteRateLimitPerHour = 1 })
	if rec := post(env.app.QuoteHandler, "/api/quote", validCompact(), true, sameOrigin); rec.Code != http.StatusOK {
		t.Fatalf("first status = %d", rec.Code)
	}
	rec := post(env.app.QuoteHandler, "/api/quote", validCompact(), true, sameOrigin)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}
}

func TestQuoteWithoutJavaScriptGetsFullPage(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := post(env.app.QuoteHandler, "/api/quote", validCompact(), false, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "Your request is in") {
		t.Error("non-htmx submission should render a complete page")
	}
}

func TestTrackWithoutIntegration(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := post(env.app.TrackHandler, "/api/track", url.Values{"tracking_number": {"pro-12345"}}, true, sameOrigin)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "PRO-12345") {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	rec = post(env.app.TrackHandler, "/api/track", url.Values{"tracking_number": {"<bad>"}}, true, sameOrigin)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid number status = %d", rec.Code)
	}
}

func TestTrackRedirectsWhenConfigured(t *testing.T) {
	env := newTestEnv(t, func(c *config.Config) { c.TrackingURL = "https://track.gfs.test/s/{ref}" })
	rec := post(env.app.TrackHandler, "/api/track", url.Values{"tracking_number": {"ABC-123"}}, true, sameOrigin)
	if got := rec.Header().Get("HX-Redirect"); got != "https://track.gfs.test/s/ABC-123" {
		t.Fatalf("HX-Redirect = %q", got)
	}
}
