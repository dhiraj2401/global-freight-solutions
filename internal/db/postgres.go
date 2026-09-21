package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store on a pgx connection pool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// PoolOptions tunes the pool for the deployment environment.
type PoolOptions struct {
	MaxConns int32
}

// NewPool creates a pool suited to serverless execution against Neon:
//
//   - Small MaxConns and zero MinConns: each function instance holds few
//     connections and releases idle ones quickly.
//   - QueryExecModeExec: avoids named server-side prepared statements, which
//     is compatible with PgBouncer transaction pooling (Neon "-pooler" hosts).
//   - The pool connects lazily, so cold starts don't pay for a round trip
//     until the first query.
func NewPool(ctx context.Context, databaseURL string, opts PoolOptions) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// Never include databaseURL in errors: it contains credentials.
		return nil, errors.New("db: invalid DATABASE_URL")
	}
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 30 * time.Second
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = "gfs-website"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	return pool, nil
}

// NewPostgresStore wraps pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// CreateQuoteRequest inserts q and returns its id.
func (s *PostgresStore) CreateQuoteRequest(ctx context.Context, q QuoteRequest) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO quote_requests (
			public_reference, first_name, last_name, company_name, email, phone,
			origin, destination, service_type, weight_kg, cargo_details,
			source, ip_hash, user_agent
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6,
			$7, $8, $9, $10, NULLIF($11, ''),
			$12, NULLIF($13, ''), NULLIF($14, '')
		)
		RETURNING id::text`,
		q.Reference, q.FirstName, q.LastName, q.CompanyName, q.Email, q.Phone,
		q.Origin, q.Destination, q.ServiceType, q.WeightKg, q.CargoDetails,
		q.Source, q.IPHash, q.UserAgent,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "public_reference") {
			return "", ErrDuplicateReference
		}
		return "", fmt.Errorf("db: insert quote request: %w", err)
	}
	return id, nil
}

// CountQuoteRequestsSince counts submissions from ipHash after since.
func (s *PostgresStore) CountQuoteRequestsSince(ctx context.Context, ipHash string, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM quote_requests WHERE ip_hash = $1 AND created_at > $2`,
		ipHash, since,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("db: count recent quote requests: %w", err)
	}
	return n, nil
}

// RecordEmailEvent inserts e.
func (s *PostgresStore) RecordEmailEvent(ctx context.Context, e EmailEvent) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO quote_email_events (
			quote_request_id, provider, provider_message_id, event_type, error_message
		) VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, ''))`,
		e.QuoteRequestID, e.Provider, e.ProviderMessageID, e.EventType, truncate(e.ErrorMessage, 2000),
	)
	if err != nil {
		return fmt.Errorf("db: record email event: %w", err)
	}
	return nil
}

// Ping checks connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Close releases pool resources.
func (s *PostgresStore) Close() { s.pool.Close() }

// migrationLockID is an arbitrary constant for pg_advisory_lock so concurrent
// migrate runs serialize instead of racing.
const migrationLockID = 7_340_001

// Migrate applies every *.sql file in dir (lexical order) that is not yet
// recorded in schema_migrations, each in its own transaction. Run it against
// a direct (non-pooled) connection: session advisory locks do not survive
// PgBouncer transaction pooling.
func Migrate(ctx context.Context, conn *pgx.Conn, fsys fs.FS, dir string) ([]string, error) {
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return nil, fmt.Errorf("migrate: acquire lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockID) //nolint:errcheck

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return nil, fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: read %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var applied []string
	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists); err != nil {
			return applied, fmt.Errorf("migrate: check %s: %w", version, err)
		}
		if exists {
			continue
		}
		sql, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return applied, fmt.Errorf("migrate: read %s: %w", name, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("migrate: begin %s: %w", version, err)
		}
		// The simple protocol allows multiple statements per file.
		if _, err := tx.Conn().PgConn().Exec(ctx, string(sql)).ReadAll(); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("migrate: apply %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("migrate: record %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("migrate: commit %s: %w", version, err)
		}
		applied = append(applied, version)
	}
	return applied, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
