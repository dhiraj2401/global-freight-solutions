package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testBrevoKey = "xkeysib-test-secret"

func newTestBrevo(t *testing.T, handler http.HandlerFunc) (*BrevoProvider, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return &BrevoProvider{
		APIKey:     testBrevoKey,
		BaseURL:    srv.URL,
		FromEmail:  "notify@gfs.test",
		FromName:   "Website",
		To:         "sales@gfs.test",
		HTTPClient: srv.Client(),
	}, &calls
}

func TestBrevoFromEnv(t *testing.T) {
	_, err := NewEmailProvider(context.Background(), "brevo", Options{To: "sales@gfs.test", Lookup: lookupFrom(nil)})
	if err == nil || !strings.Contains(err.Error(), "BREVO_API_KEY") {
		t.Fatalf("err = %v, want missing BREVO_API_KEY", err)
	}

	_, err = NewEmailProvider(context.Background(), "brevo", Options{To: "sales@gfs.test", Lookup: lookupFrom(map[string]string{
		"BREVO_SMTP_KEY": "xsmtpsib-old", "BREVO_FROM_EMAIL": "notify@gfs.test",
	})})
	if err == nil || !strings.Contains(err.Error(), "no longer used") {
		t.Errorf("err = %v, want migration hint for old SMTP configuration", err)
	}

	p, err := NewEmailProvider(context.Background(), "brevo", Options{To: "sales@gfs.test", Lookup: lookupFrom(map[string]string{
		"BREVO_API_KEY": testBrevoKey, "BREVO_FROM_EMAIL": "notify@gfs.test",
	})})
	if err != nil {
		t.Fatal(err)
	}
	if b := p.(*BrevoProvider); b.BaseURL != "https://api.brevo.com/v3" || b.APIKey != testBrevoKey {
		t.Errorf("provider = %+v", b)
	}
}

func TestBrevoSendsTransactionalEmail(t *testing.T) {
	var (
		got    brevoSendRequest
		header http.Header
		method string
		path   string
	)
	p, calls := newTestBrevo(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, header = r.Method, r.URL.Path, r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"messageId":"<202609141200.1234@smtp-relay.mailin.fr>"}`)
	})

	res := p.SendQuoteNotification(context.Background(), sampleNotification())
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Provider != "brevo" || res.MessageID != "<202609141200.1234@smtp-relay.mailin.fr>" {
		t.Errorf("result = %+v", res)
	}
	if calls.Load() != 1 || method != http.MethodPost || path != "/smtp/email" {
		t.Errorf("calls=%d method=%s path=%s", calls.Load(), method, path)
	}
	if header.Get("api-key") != testBrevoKey || header.Get("Content-Type") != "application/json" {
		t.Errorf("headers = %v", header)
	}
	if got.Sender.Email != "notify@gfs.test" || got.Sender.Name != "Jane Doe (jane@acme.test)" {
		t.Errorf("sender = %+v", got.Sender)
	}
	if len(got.To) != 1 || got.To[0].Email != "sales@gfs.test" {
		t.Errorf("to = %+v", got.To)
	}
	if got.ReplyTo == nil || got.ReplyTo.Email != "jane@acme.test" || got.ReplyTo.Name != "Jane Doe" {
		t.Errorf("replyTo = %+v", got.ReplyTo)
	}
	if !strings.Contains(got.Subject, "GFS-260914-ABCDEF") || !strings.Contains(got.HTMLContent, "Houston, TX") || got.TextContent == "" {
		t.Errorf("content not rendered: subject=%q", got.Subject)
	}
	if got.Headers["X-Quote-Reference"] != "GFS-260914-ABCDEF" {
		t.Errorf("headers = %v", got.Headers)
	}
}

func TestBrevoAPIErrorIsDescriptiveAndHidesKey(t *testing.T) {
	p, calls := newTestBrevo(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"code":"unauthorized","message":"Key not found"}`)
	})
	res := p.SendQuoteNotification(context.Background(), sampleNotification())
	if res.Err == nil {
		t.Fatal("expected error")
	}
	msg := res.Err.Error()
	for _, want := range []string{"401", "unauthorized", "Key not found"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	if strings.Contains(msg, testBrevoKey) {
		t.Error("error leaks the API key")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, client errors must not be retried", calls.Load())
	}
}

func TestBrevoRetriesRateLimitOnce(t *testing.T) {
	var attempts atomic.Int32
	p, calls := newTestBrevo(t, func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"messageId":"<ok@brevo>"}`)
	})
	res := p.SendQuoteNotification(context.Background(), sampleNotification())
	if res.Err != nil || res.MessageID != "<ok@brevo>" || calls.Load() != 2 {
		t.Fatalf("result = %+v, calls = %d", res, calls.Load())
	}
}

func TestBrevoGivesUpAfterRepeatedRateLimit(t *testing.T) {
	p, calls := newTestBrevo(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	res := p.SendQuoteNotification(context.Background(), sampleNotification())
	if res.Err == nil || calls.Load() != 2 {
		t.Fatalf("err = %v, calls = %d, want error after 2 attempts", res.Err, calls.Load())
	}
}

func TestBrevoHonoursContextDeadline(t *testing.T) {
	p, _ := newTestBrevo(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	res := p.SendQuoteNotification(ctx, sampleNotification())
	if res.Err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("send took %s, deadline not honoured", elapsed)
	}
}
