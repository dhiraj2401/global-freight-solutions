package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDevelopmentDefaults(t *testing.T) {
	cfg, err := LoadFrom(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != EnvDevelopment || cfg.EmailProvider != "log" || cfg.Port != "6162" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) == 0 || cfg.AllowedOrigins[0] != "http://localhost:6162" {
		t.Errorf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
	if cfg.TrustProxyHeaders {
		t.Error("proxy headers must not be trusted by default outside Vercel")
	}
}

func TestProductionRequiresSettings(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{"APP_ENV": "production"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, key := range []string{"DATABASE_URL", "EMAIL_PROVIDER", "SALES_NOTIFICATION_EMAIL", "IP_HASH_SALT", "ALLOWED_ORIGIN"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

func TestProductionRejectsLogProvider(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{
		"APP_ENV":                  "production",
		"DATABASE_URL":             "postgres://x",
		"EMAIL_PROVIDER":           "log",
		"SALES_NOTIFICATION_EMAIL": "sales@gfs.test",
		"IP_HASH_SALT":             "0123456789abcdef0123",
		"ALLOWED_ORIGIN":           "https://gfs.test",
	}))
	if err == nil || !strings.Contains(err.Error(), "log") {
		t.Fatalf("err = %v, want log provider rejection", err)
	}
}

func TestVercelDefaults(t *testing.T) {
	cfg, err := LoadFrom(env(map[string]string{
		"VERCEL":                   "1",
		"DATABASE_URL":             "postgres://x",
		"EMAIL_PROVIDER":           "ses",
		"SALES_NOTIFICATION_EMAIL": "sales@gfs.test",
		"IP_HASH_SALT":             "0123456789abcdef0123",
		"ALLOWED_ORIGIN":           "https://gfs.test, https://www.gfs.test/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.IsProduction() || !cfg.TrustProxyHeaders {
		t.Errorf("Vercel should imply production and trusted proxy headers: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[1] != "https://www.gfs.test" {
		t.Errorf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
}

func TestInvalidValues(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{
		"ALLOWED_ORIGIN":            "gfs.test",
		"QUOTE_RATE_LIMIT_PER_HOUR": "lots",
		"TRACKING_URL":              "http://insecure.test/{ref}",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"ALLOWED_ORIGIN", "QUOTE_RATE_LIMIT_PER_HOUR", "TRACKING_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}
