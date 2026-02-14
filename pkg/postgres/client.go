// Package postgres provides a thin wrapper around database/sql with
// connection-pool configuration, health-check support, and a transactional
// helper (InTx).
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	_ "github.com/lib/pq"
)

// Client manages a PostgreSQL connection pool.
type Client struct {
	DB  *sql.DB
	cfg config.PostgresConfig
}

// New opens a PostgreSQL connection pool, configures its limits, and pings
// the server to verify connectivity.
func New(cfg config.PostgresConfig) (*Client, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}
	return &Client{DB: db, cfg: cfg}, nil
}

// Close releases the database connection pool.
func (c *Client) Close() error {
	return c.DB.Close()
}

// InTx executes fn inside a database transaction. On error the transaction is
// rolled back; on success it is committed.
func (c *Client) InTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rolling back transaction after error %v: %w", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
