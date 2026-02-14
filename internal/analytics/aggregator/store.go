// Package aggregator provides persistent storage and periodic snapshotting
// of aggregated analytics stats to PostgreSQL.
package aggregator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/analytics"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// Store persists aggregated analytics snapshots in PostgreSQL.
//
// It requires an `analytics_snapshots` table:
//
//	CREATE TABLE analytics_snapshots (
//	    id         BIGSERIAL PRIMARY KEY,
//	    data       JSONB NOT NULL,
//	    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
//	);
type Store struct {
	db     *postgres.Client
	logger *slog.Logger
}

// NewStore creates a new analytics persistence store.
func NewStore(db *postgres.Client) *Store {
	return &Store{
		db:     db,
		logger: slog.Default().With("component", "analytics-store"),
	}
}

// SaveSnapshot persists a stats snapshot to the database.
func (s *Store) SaveSnapshot(ctx context.Context, stats analytics.AggregatedStats) error {
	data, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("marshaling stats: %w", err)
	}

	_, err = s.db.DB.ExecContext(ctx,
		`INSERT INTO analytics_snapshots (data, captured_at) VALUES ($1, $2)`,
		data, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("saving analytics snapshot: %w", err)
	}

	s.logger.Info("analytics snapshot saved",
		"total_searches", stats.TotalSearches,
		"total_docs_indexed", stats.TotalDocIndexed,
	)
	return nil
}

// LatestSnapshot loads the most recent snapshot from the database.
// Returns nil, nil if no snapshots exist yet.
func (s *Store) LatestSnapshot(ctx context.Context) (*analytics.AggregatedStats, error) {
	var data []byte
	err := s.db.DB.QueryRowContext(ctx,
		`SELECT data FROM analytics_snapshots ORDER BY captured_at DESC LIMIT 1`,
	).Scan(&data)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying latest snapshot: %w", err)
	}

	var stats analytics.AggregatedStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("unmarshaling snapshot: %w", err)
	}
	return &stats, nil
}

// ListSnapshots returns the last N snapshots, newest first.
func (s *Store) ListSnapshots(ctx context.Context, limit int) ([]analytics.AggregatedStats, error) {
	rows, err := s.db.DB.QueryContext(ctx,
		`SELECT data FROM analytics_snapshots ORDER BY captured_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []analytics.AggregatedStats
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scanning snapshot row: %w", err)
		}
		var stats analytics.AggregatedStats
		if err := json.Unmarshal(data, &stats); err != nil {
			s.logger.Warn("skipping corrupt snapshot", "error", err)
			continue
		}
		snapshots = append(snapshots, stats)
	}

	return snapshots, rows.Err()
}

// StartPeriodicSave launches a goroutine that periodically snapshots
// the aggregator's current stats to the database.
func (s *Store) StartPeriodicSave(ctx context.Context, agg *analytics.Aggregator, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := agg.Stats()
				if err := s.SaveSnapshot(ctx, stats); err != nil {
					s.logger.Error("periodic snapshot failed", "error", err)
				}
			case <-ctx.Done():
				// Final snapshot on shutdown.
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				stats := agg.Stats()
				if err := s.SaveSnapshot(shutdownCtx, stats); err != nil {
					s.logger.Error("final snapshot failed", "error", err)
				}
				return
			}
		}
	}()
	s.logger.Info("periodic snapshot started", "interval", interval)
}
