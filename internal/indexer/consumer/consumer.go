// Package consumer reads ingestion events from Kafka and indexes them
// via the indexer engine, optionally routing documents through the shard
// router for partitioned indexing.
package consumer

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/shard"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/ingestion"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
)

// IndexConsumer wraps a Kafka consumer to drive the indexing pipeline.
type IndexConsumer struct {
	consumer *kafka.Consumer
	logger   *slog.Logger
}

// New creates an IndexConsumer backed by the given Kafka consumer.
func New(kafkaConsumer *kafka.Consumer) *IndexConsumer {
	return &IndexConsumer{
		consumer: kafkaConsumer,
		logger:   slog.Default().With("component", "index-consumer"),
	}
}

// Start begins consuming Kafka messages. It blocks until ctx is cancelled.
func (ic *IndexConsumer) Start(ctx context.Context) error {
	ic.logger.Info("index consumer starting")
	return ic.consumer.Start(ctx)
}

// HandleMessageSharded returns a Kafka MessageHandler that routes each ingest
// event to the correct shard engine via the Router before indexing.
// If db is non-nil, the document status is updated from PENDING to INDEXED
// in PostgreSQL after a successful index operation.
func HandleMessageSharded(router *shard.Router, db *sql.DB) kafka.MessageHandler {
	logger := slog.Default().With("component", "index-consumer")
	return func(ctx context.Context, key []byte, value []byte) error {
		event, err := kafka.DecodeJSON[ingestion.IngestEvent](value)
		if err != nil {
			logger.Error("failed to decode ingest event",
				"error", err,
				"key", string(key),
			)
			return nil
		}

		engine, err := router.Route(event.ShardID)
		if err != nil {
			return fmt.Errorf("routing shard %d: %w", event.ShardID, err)
		}

		logger.Debug("processing ingest event",
			"doc_id", event.DocumentID,
			"shard_id", event.ShardID,
		)

		if err := engine.IndexDocument(event.DocumentID, event.Title, event.Body); err != nil {
			updateDocStatus(ctx, db, event.DocumentID, "FAILED", logger)
			return fmt.Errorf("indexing document %s in shard %d: %w", event.DocumentID, event.ShardID, err)
		}

		updateDocStatus(ctx, db, event.DocumentID, "INDEXED", logger)

		logger.Info("document indexed",
			"doc_id", event.DocumentID,
			"shard_id", event.ShardID,
		)
		return nil
	}
}

// HandleMessage returns a Kafka MessageHandler that indexes every ingest
// event into a single (non-sharded) Engine.
// If db is non-nil, the document status is updated after indexing.
func HandleMessage(engine *indexer.Engine, db *sql.DB) kafka.MessageHandler {
	logger := slog.Default().With("component", "index-consumer")
	return func(ctx context.Context, key []byte, value []byte) error {
		event, err := kafka.DecodeJSON[ingestion.IngestEvent](value)
		if err != nil {
			logger.Error("failed to decode ingest event",
				"error", err,
				"key", string(key),
			)
			return nil
		}
		logger.Debug("processing ingest event",
			"doc_id", event.DocumentID,
			"shard_id", event.ShardID,
		)
		if err := engine.IndexDocument(event.DocumentID, event.Title, event.Body); err != nil {
			updateDocStatus(ctx, db, event.DocumentID, "FAILED", logger)
			return fmt.Errorf("indexing document %s: %w", event.DocumentID, err)
		}

		updateDocStatus(ctx, db, event.DocumentID, "INDEXED", logger)

		logger.Info("document indexed",
			"doc_id", event.DocumentID,
			"shard_id", event.ShardID,
		)
		return nil
	}
}

// updateDocStatus updates the document's status and indexed_at timestamp in PostgreSQL.
// If db is nil, the update is silently skipped.
func updateDocStatus(ctx context.Context, db *sql.DB, docID, status string, logger *slog.Logger) {
	if db == nil {
		return
	}
	_, err := db.ExecContext(ctx,
		`UPDATE documents SET status = $1, indexed_at = NOW() WHERE id = $2`,
		status, docID,
	)
	if err != nil {
		logger.Error("failed to update document status",
			"doc_id", docID,
			"status", status,
			"error", err,
		)
	}
}
