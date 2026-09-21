package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register("brevo", newBrevoFromEnv)
}

const (
	defaultBrevoBaseURL = "https://api.brevo.com/v3"
	brevoMaxAttempts    = 2
	brevoDefaultBackoff = 500 * time.Millisecond
	brevoMaxBody        = 64 << 10
)

// brevoHTTPClient is shared so warm serverless invocations reuse TLS
// connections. Per-request deadlines come from the caller's context.
var brevoHTTPClient = &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}

// BrevoProvider delivers notifications through Brevo's transactional email
// REST API (POST /v3/smtp/email).
//
// Compared with SMTP this suits serverless runtimes: one HTTPS request instead
// of a multi-step SMTP conversation, no outbound port 587, connection reuse
// across warm invocations, structured errors and a provider message ID.
type BrevoProvider struct {
	APIKey    string
	BaseURL   string // defaults to https://api.brevo.com/v3
	FromEmail string
	FromName  string
	To        string

	// HTTPClient may be replaced in tests. Defaults to a shared client.
	HTTPClient *http.Client
}

func newBrevoFromEnv(_ context.Context, opts Options) (EmailProvider, error) {
	if err := requireEnv(opts.Lookup, "BREVO_API_KEY", "BREVO_FROM_EMAIL"); err != nil {
		if strings.TrimSpace(opts.Lookup("BREVO_API_KEY")) == "" && strings.TrimSpace(opts.Lookup("BREVO_SMTP_KEY")) != "" {
			err = fmt.Errorf("%w (BREVO_SMTP_KEY is no longer used: the provider now calls the Brevo REST API and needs an API key)", err)
		}
		return nil, fmt.Errorf("brevo: %w", err)
	}
	from := strings.TrimSpace(opts.Lookup("BREVO_FROM_EMAIL"))
	if _, err := mail.ParseAddress(from); err != nil {
		return nil, fmt.Errorf("brevo: invalid BREVO_FROM_EMAIL: %w", err)
	}
	return &BrevoProvider{
		APIKey:    strings.TrimSpace(opts.Lookup("BREVO_API_KEY")),
		BaseURL:   defaultBrevoBaseURL,
		FromEmail: from,
		FromName:  opts.FromName,
		To:        opts.To,
	}, nil
}

// Name implements EmailProvider.
func (b *BrevoProvider) Name() string { return "brevo" }

// BrevoAPIError is a non-2xx response from the Brevo API.
type BrevoAPIError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter time.Duration
	hasRetry   bool
}

func (e *BrevoAPIError) Error() string {
	msg := fmt.Sprintf("status %d", e.StatusCode)
	if e.Code != "" {
		msg += " " + e.Code
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

// retryable reports whether the request was certainly not accepted, so a
// retry cannot produce a duplicate email (Brevo has no idempotency keys).
func (e *BrevoAPIError) retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode == http.StatusServiceUnavailable
}

type brevoAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type brevoSendRequest struct {
	Sender      brevoAddress      `json:"sender"`
	To          []brevoAddress    `json:"to"`
	ReplyTo     *brevoAddress     `json:"replyTo,omitempty"`
	Subject     string            `json:"subject"`
	HTMLContent string            `json:"htmlContent"`
	TextContent string            `json:"textContent"`
	Headers     map[string]string `json:"headers,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// SendQuoteNotification implements EmailProvider.
func (b *BrevoProvider) SendQuoteNotification(ctx context.Context, notif QuoteNotification) SendResult {
	res := SendResult{Provider: b.Name()}
	msg, err := BuildMessage(Envelope{From: mail.Address{Name: b.FromName, Address: b.FromEmail}, To: b.To}, notif)
	if err != nil {
		res.Err = err
		return res
	}

	payload := brevoSendRequest{
		Sender:      brevoAddress{Email: msg.From.Address, Name: msg.From.Name},
		To:          []brevoAddress{{Email: msg.To.Address, Name: msg.To.Name}},
		Subject:     msg.Subject,
		HTMLContent: msg.HTML,
		TextContent: msg.Text,
		Tags:        []string{"quote-notification"},
	}
	if msg.ReplyTo != nil {
		payload.ReplyTo = &brevoAddress{Email: msg.ReplyTo.Address, Name: msg.ReplyTo.Name}
	}
	if ref := headerSafe(notif.Reference, 64); ref != "" {
		payload.Headers = map[string]string{"X-Quote-Reference": ref}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		res.Err = fmt.Errorf("brevo api: encode request: %w", err)
		return res
	}

	res.MessageID, res.Err = b.send(ctx, body)
	if res.Err != nil {
		res.Err = fmt.Errorf("brevo api: %w", res.Err)
	}
	return res
}

func (b *BrevoProvider) send(ctx context.Context, body []byte) (string, error) {
	for attempt := 1; ; attempt++ {
		id, err := b.post(ctx, body)
		if err == nil {
			return id, nil
		}
		var apiErr *BrevoAPIError
		if attempt >= brevoMaxAttempts || !errors.As(err, &apiErr) || !apiErr.retryable() {
			return "", err
		}
		wait := brevoDefaultBackoff
		if apiErr.hasRetry {
			wait = apiErr.RetryAfter
		}
		// Don't start a retry that cannot finish inside the send deadline.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < wait+time.Second {
			return "", err
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", err
		case <-timer.C:
		}
	}
}

func (b *BrevoProvider) post(ctx context.Context, body []byte) (string, error) {
	base := strings.TrimRight(b.BaseURL, "/")
	if base == "" {
		base = defaultBrevoBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/smtp/email", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("api-key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := b.HTTPClient
	if client == nil {
		client = brevoHTTPClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, brevoMaxBody))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out struct {
			MessageID string `json:"messageId"`
		}
		// A 2xx means Brevo accepted the email; a missing ID is not a failure.
		_ = json.Unmarshal(data, &out)
		return out.MessageID, nil
	}

	apiErr := &BrevoAPIError{StatusCode: resp.StatusCode}
	var errBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &errBody) == nil {
		apiErr.Code, apiErr.Message = errBody.Code, errBody.Message
	}
	if apiErr.Code == "" && apiErr.Message == "" {
		apiErr.Message = headerSafe(string(data), 300)
	}
	if raw := resp.Header.Get("Retry-After"); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs >= 0 {
			apiErr.RetryAfter, apiErr.hasRetry = time.Duration(secs)*time.Second, true
		}
	}
	return "", apiErr
}
