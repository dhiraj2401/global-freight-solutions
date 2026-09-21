-- 001_initial_schema.sql
-- Quote requests captured from the website and the email notifications sent
-- for them. Applied by `go run ./cmd/migrate` (tracked in schema_migrations).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE quote_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_reference TEXT NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    last_name TEXT,
    company_name TEXT,
    email TEXT NOT NULL,
    phone TEXT NOT NULL,
    origin TEXT NOT NULL,
    destination TEXT NOT NULL,
    service_type TEXT NOT NULL,
    weight_kg NUMERIC(12, 2) CHECK (weight_kg IS NULL OR weight_kg > 0),
    cargo_details TEXT,
    status TEXT NOT NULL DEFAULT 'new'
        CHECK (status IN (
            'new', 'contacted', 'qualified',
            'quoted', 'won', 'lost', 'invalid'
        )),
    assigned_to TEXT,
    source TEXT NOT NULL DEFAULT 'website',
    ip_hash TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    contacted_at TIMESTAMPTZ,
    quoted_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ
);

CREATE TABLE quote_email_events (
    id BIGSERIAL PRIMARY KEY,
    quote_request_id UUID NOT NULL
        REFERENCES quote_requests(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_message_id TEXT,
    event_type TEXT NOT NULL,
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX quote_requests_created_at_idx
    ON quote_requests (created_at DESC);
CREATE INDEX quote_requests_status_idx
    ON quote_requests (status);
CREATE INDEX quote_requests_email_idx
    ON quote_requests (email);
-- Supports the per-client submission rate limit.
CREATE INDEX quote_requests_ip_hash_created_at_idx
    ON quote_requests (ip_hash, created_at DESC)
    WHERE ip_hash IS NOT NULL;
CREATE INDEX quote_email_events_quote_request_id_idx
    ON quote_email_events (quote_request_id);

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER quote_requests_set_updated_at
    BEFORE UPDATE ON quote_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
