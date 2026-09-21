// Command migrate applies pending SQL migrations from db/migrations.
//
// It prefers DATABASE_URL_UNPOOLED (set by the Neon ↔ Vercel integration)
// because migrations take a session-level advisory lock, which PgBouncer
// transaction pooling does not preserve.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	gfs "github.com/gloitel/global-freight-solutions"
	"github.com/gloitel/global-freight-solutions/internal/config"
	"github.com/gloitel/global-freight-solutions/internal/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	url := os.Getenv("DATABASE_URL_UNPOOLED")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return errors.New("set DATABASE_URL_UNPOOLED or DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		// pgx errors do not include the password, but avoid echoing the URL.
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(context.Background())

	applied, err := db.Migrate(ctx, conn, gfs.Migrations(), "db/migrations")
	for _, v := range applied {
		fmt.Println("applied", v)
	}
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Println("database is up to date")
	}
	return nil
}
