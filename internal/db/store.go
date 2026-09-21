// Package db persists quote requests. Postgres (Neon) is used in production;
// an in-memory store lets the site run locally without a database.
package db

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"
)

// ErrDuplicateReference is returned when a generated public reference
// collides with an existing one. Callers should generate a new one and retry.
var ErrDuplicateReference = errors.New("db: duplicate public reference")

// QuoteRequest is a validated submission ready to persist.
type QuoteRequest struct {
	Reference    string
	FirstName    string
	LastName     string
	CompanyName  string
	Email        string
	Phone        string
	Origin       string
	Destination  string
	ServiceType  string
	WeightKg     *float64
	CargoDetails string
	Source       string
	IPHash       string
	UserAgent    string
}

// EmailEvent records the outcome of a notification send.
type EmailEvent struct {
	QuoteRequestID    string
	Provider          string
	ProviderMessageID string
	EventType         string // "sent" or "failed"
	ErrorMessage      string
}

// Store is the persistence contract used by HTTP handlers.
type Store interface {
	CreateQuoteRequest(ctx context.Context, q QuoteRequest) (id string, err error)
	CountQuoteRequestsSince(ctx context.Context, ipHash string, since time.Time) (int, error)
	RecordEmailEvent(ctx context.Context, e EmailEvent) error
	Ping(ctx context.Context) error
	Close()
}

// referenceAlphabet omits 0/O and 1/I so references are easy to read aloud.
const referenceAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// NewReference returns a human-friendly reference such as GFS-260914-K7QX9M.
func NewReference(prefix string, now time.Time) string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	var b strings.Builder
	b.WriteString(strings.ToUpper(prefix))
	b.WriteByte('-')
	b.WriteString(now.UTC().Format("060102"))
	b.WriteByte('-')
	for _, v := range buf {
		b.WriteByte(referenceAlphabet[int(v)%len(referenceAlphabet)])
	}
	return b.String()
}
