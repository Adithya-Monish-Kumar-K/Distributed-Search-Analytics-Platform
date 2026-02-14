// Package publisher persists documents to PostgreSQL and publishes ingest
// events to Kafka for downstream indexing. It performs content-hash-based
// shard assignment and supports idempotent writes.
package publisher

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/ingestion"
	apperrors "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/errors"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// totalShards is the fixed number of index shards used for partitioning.
const totalShards = 8

// Publisher coordinates document persistence and Kafka event production.
type Publisher struct {
	db       *postgres.Client
	producer *kafka.Producer
	logger   *slog.Logger
}

// New creates a Publisher with the given database and Kafka producer.
func New(db *postgres.Client, producer *kafka.Producer) *Publisher {
	return &Publisher{
		db:       db,
		producer: producer,
		logger:   slog.Default().With("component", "publisher"),
	}
}

// Ingest persists the document in PostgreSQL, assigns a shard, and publishes
// an IngestEvent to Kafka. Duplicate idempotency keys are detected and
// returned without re-insertion.
func (p *Publisher) Ingest(ctx context.Context, req *ingestion.IngestRequest) (*ingestion.IngestResponse, error) {
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Body)))
	if req.IdempotencyKey != "" {
		existing, err := p.findByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("checking idempotency key: %w", err)
		}
		if existing != nil {
			p.logger.Info("duplicate ingestion detected",
				"idempotency_key", req.IdempotencyKey,
				"existing_id", existing.DocumentID,
			)
			return existing, nil
		}
	}

	shardID := assignShard(contentHash, totalShards)
	var docID string
	err := p.db.InTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO documents (title, content_hash, content_size, shard_id, idempotency_key, status)
		VALUES ($1, $2, $3, $4, $5, 'PENDING')
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`, req.Title, contentHash, len(req.Body), shardID, nullableString(req.IdempotencyKey)).Scan(&docID)
		if err == sql.ErrNoRows {
			return apperrors.New(apperrors.ErrIdempotencyConflict, 409, "idempotency key already in use")
		}
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("inserting document: %w", err)
	}

	event := kafka.Event{
		Key: strconv.Itoa(shardID),
		Value: ingestion.IngestEvent{
			DocumentID: docID,
			Title:      req.Title,
			Body:       req.Body,
			ShardID:    shardID,
			IngestedAt: time.Now().UTC(),
		},
	}

	if err := p.producer.Publish(ctx, event); err != nil {
		p.logger.Error("failed to publish to kafka, document stuck in PENDING",
			"doc_id", docID,
			"shard_id", shardID,
			"error", err,
		)
	}
	return &ingestion.IngestResponse{
		DocumentID: docID,
		Status:     "PENDING",
		ShardID:    shardID,
	}, nil
}

// findByIdempotencyKey checks if a document with the given idempotency key
// already exists and returns its status.
func (p *Publisher) findByIdempotencyKey(ctx context.Context, key string) (*ingestion.IngestResponse, error) {
	var resp ingestion.IngestResponse
	err := p.db.DB.QueryRowContext(ctx,
		`SELECT id, status, shard_id FROM documents WHERE idempotency_key=$1`, key).Scan(&resp.DocumentID, &resp.Status, &resp.ShardID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying by idempotency key: %w", err)
	}
	return &resp, nil
}

// assignShard deterministically maps a content hash to a shard ID.
func assignShard(contentHash string, numShards int) int {
	var hash uint64
	for i := 0; i < 8 && i < len(contentHash); i++ {
		hash = hash<<8 | uint64(contentHash[i])
	}
	return int(hash % uint64(numShards))
}

// nullableString converts a Go string to a sql.NullString, treating the
// empty string as NULL.
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
