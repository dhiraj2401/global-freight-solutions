package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// MemoryStore is a non-persistent Store for local development and tests.
type MemoryStore struct {
	mu     sync.Mutex
	quotes []memoryQuote
	events []EmailEvent
}

type memoryQuote struct {
	id        string
	createdAt time.Time
	QuoteRequest
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// CreateQuoteRequest implements Store.
func (m *MemoryStore) CreateQuoteRequest(_ context.Context, q QuoteRequest) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.quotes {
		if existing.Reference == q.Reference {
			return "", ErrDuplicateReference
		}
	}
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	id := hex.EncodeToString(buf)
	m.quotes = append(m.quotes, memoryQuote{id: id, createdAt: time.Now(), QuoteRequest: q})
	return id, nil
}

// CountQuoteRequestsSince implements Store.
func (m *MemoryStore) CountQuoteRequestsSince(_ context.Context, ipHash string, since time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, q := range m.quotes {
		if q.IPHash == ipHash && q.createdAt.After(since) {
			n++
		}
	}
	return n, nil
}

// RecordEmailEvent implements Store.
func (m *MemoryStore) RecordEmailEvent(_ context.Context, e EmailEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

// Quotes returns a copy of stored requests (for tests).
func (m *MemoryStore) Quotes() []QuoteRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]QuoteRequest, len(m.quotes))
	for i, q := range m.quotes {
		out[i] = q.QuoteRequest
	}
	return out
}

// Events returns a copy of recorded email events (for tests).
func (m *MemoryStore) Events() []EmailEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]EmailEvent(nil), m.events...)
}

// Ping implements Store.
func (m *MemoryStore) Ping(context.Context) error { return nil }

// Close implements Store.
func (m *MemoryStore) Close() {}
