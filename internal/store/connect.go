package store

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// Connect opens a connection pool to Postgres and runs the schema
// so tables are created if they don't already exist.
func Connect(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		return nil, fmt.Errorf("running schema: %w", err)
	}
	return pool, nil
}
