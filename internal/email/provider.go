// Package email defines a provider-agnostic contract for transactional email
// and ships pluggable implementations (Brevo SMTP, AWS SES, and a local log
// provider). Providers self-register in init(), so selecting one is purely a
// matter of setting EMAIL_PROVIDER.
package email

import (
	"context"
	"time"
)

// QuoteNotification is the payload sent to the sales inbox for every new
// quote request.
type QuoteNotification struct {
	Reference    string
	FirstName    string
	LastName     string
	Company      string
	Email        string
	Phone        string
	Origin       string
	Destination  string
	ServiceType  string
	WeightKg     float64
	CargoDetails string
	SubmittedAt  time.Time
}

// SendResult reports the outcome of a send. Err is nil on success.
type SendResult struct {
	Provider  string
	MessageID string
	Err       error
}

// EmailProvider is implemented by every email backend.
//
// Implementations must be safe for concurrent use and must honour ctx
// cancellation and deadlines.
type EmailProvider interface {
	SendQuoteNotification(ctx context.Context, notif QuoteNotification) SendResult
	Name() string
}

// Options carries the settings shared by all providers. Provider-specific
// settings are read through Lookup so that adding a provider never requires
// touching central configuration code.
type Options struct {
	// To is the sales notification inbox (SALES_NOTIFICATION_EMAIL).
	To string
	// FromName is the display name used in the From header (EMAIL_FROM_NAME).
	FromName string
	// Lookup returns the value of an environment variable ("" if unset).
	Lookup func(key string) string
}

// Factory constructs a provider from Options.
type Factory func(ctx context.Context, opts Options) (EmailProvider, error)
