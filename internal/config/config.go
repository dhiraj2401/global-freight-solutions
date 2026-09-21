// Package config loads runtime configuration from environment variables.
//
// Only settings shared across the app live here. Email providers read their
// own variables (see internal/email), and site copy lives in internal/content.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// Config holds application settings.
type Config struct {
	Env  string
	Port string

	// AllowedOrigins are the exact origins (scheme://host[:port]) permitted to
	// POST forms. Used for CSRF protection; there is no cross-origin API.
	AllowedOrigins []string

	DatabaseURL string
	DBMaxConns  int32

	EmailProvider          string
	EmailFromName          string
	EmailSendTimeout       time.Duration
	SalesNotificationEmail string

	IPHashSalt            string
	TrustProxyHeaders     bool
	QuoteRateLimitPerHour int

	// TrackingURL is an optional external tracking page. "{ref}" is replaced
	// with the URL-escaped tracking number.
	TrackingURL string

	// Lookup exposes the environment to pluggable components.
	Lookup func(string) string
}

// IsProduction reports whether the app runs with production safeguards.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Load reads configuration from the process environment.
func Load() (Config, error) { return LoadFrom(os.Getenv) }

// LoadFrom reads configuration using lookup, which makes it testable.
func LoadFrom(lookup func(string) string) (Config, error) {
	get := func(key string) string { return strings.TrimSpace(lookup(key)) }

	env := strings.ToLower(get("APP_ENV"))
	if env == "" {
		if get("VERCEL") == "1" {
			env = EnvProduction
		} else {
			env = EnvDevelopment
		}
	}

	cfg := Config{
		Env:                    env,
		Port:                   valueOr(get("PORT"), "6162"),
		DatabaseURL:            get("DATABASE_URL"),
		EmailProvider:          strings.ToLower(get("EMAIL_PROVIDER")),
		EmailFromName:          valueOr(get("EMAIL_FROM_NAME"), "Website Quote Requests"),
		SalesNotificationEmail: get("SALES_NOTIFICATION_EMAIL"),
		IPHashSalt:             get("IP_HASH_SALT"),
		TrackingURL:            get("TRACKING_URL"),
		Lookup:                 lookup,
	}

	var errs []error
	if env != EnvDevelopment && env != EnvProduction {
		errs = append(errs, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, env))
	}

	var err error
	if cfg.DBMaxConns, err = intVar[int32](get, "DB_MAX_CONNS", 4); err != nil {
		errs = append(errs, err)
	}
	if cfg.QuoteRateLimitPerHour, err = intVar(get, "QUOTE_RATE_LIMIT_PER_HOUR", 5); err != nil {
		errs = append(errs, err)
	}
	timeoutSeconds, err := intVar(get, "EMAIL_SEND_TIMEOUT_SECONDS", 8)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.EmailSendTimeout = time.Duration(timeoutSeconds) * time.Second

	// Vercel terminates TLS and sets X-Real-IP / X-Forwarded-For itself, so
	// those headers are trustworthy there. Elsewhere, opt in explicitly.
	cfg.TrustProxyHeaders = get("VERCEL") == "1"
	if raw := get("TRUST_PROXY_HEADERS"); raw != "" {
		v, perr := strconv.ParseBool(raw)
		if perr != nil {
			errs = append(errs, fmt.Errorf("TRUST_PROXY_HEADERS: %w", perr))
		}
		cfg.TrustProxyHeaders = v
	}

	for _, origin := range strings.Split(get("ALLOWED_ORIGIN"), ",") {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "" {
			continue
		}
		u, perr := url.Parse(origin)
		if perr != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
			errs = append(errs, fmt.Errorf("ALLOWED_ORIGIN: %q is not a valid origin (expected scheme://host)", origin))
			continue
		}
		cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
	}

	if cfg.SalesNotificationEmail != "" {
		if _, perr := mail.ParseAddress(cfg.SalesNotificationEmail); perr != nil {
			errs = append(errs, fmt.Errorf("SALES_NOTIFICATION_EMAIL: %w", perr))
		}
	}
	if cfg.TrackingURL != "" {
		if u, perr := url.Parse(cfg.TrackingURL); perr != nil || u.Scheme != "https" {
			errs = append(errs, errors.New("TRACKING_URL must be an absolute https URL"))
		}
	}

	if cfg.IsProduction() {
		required := map[string]string{
			"DATABASE_URL":             cfg.DatabaseURL,
			"EMAIL_PROVIDER":           cfg.EmailProvider,
			"SALES_NOTIFICATION_EMAIL": cfg.SalesNotificationEmail,
			"IP_HASH_SALT":             cfg.IPHashSalt,
			"ALLOWED_ORIGIN":           strings.Join(cfg.AllowedOrigins, ","),
		}
		for _, key := range []string{"DATABASE_URL", "EMAIL_PROVIDER", "SALES_NOTIFICATION_EMAIL", "IP_HASH_SALT", "ALLOWED_ORIGIN"} {
			if required[key] == "" {
				errs = append(errs, fmt.Errorf("%s is required in production", key))
			}
		}
		if cfg.EmailProvider == "log" {
			errs = append(errs, errors.New(`EMAIL_PROVIDER="log" does not send email and is not allowed in production`))
		}
		if cfg.IPHashSalt != "" && len(cfg.IPHashSalt) < 16 {
			errs = append(errs, errors.New("IP_HASH_SALT must be at least 16 characters"))
		}
	} else {
		// Development defaults let `go run ./cmd/server` work with no setup.
		if cfg.EmailProvider == "" {
			cfg.EmailProvider = "log"
		}
		if cfg.SalesNotificationEmail == "" {
			cfg.SalesNotificationEmail = "sales@example.com"
		}
		if cfg.IPHashSalt == "" {
			cfg.IPHashSalt = "development-only-insecure-salt"
		}
		if len(cfg.AllowedOrigins) == 0 {
			cfg.AllowedOrigins = []string{"http://localhost:" + cfg.Port, "http://127.0.0.1:" + cfg.Port}
		}
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func intVar[T int | int32](get func(string) string, key string, def T) (T, error) {
	raw := get(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def, fmt.Errorf("%s must be a positive integer, got %q", key, raw)
	}
	return T(v), nil
}

func valueOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
