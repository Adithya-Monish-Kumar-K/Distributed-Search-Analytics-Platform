// Command indexer starts the document indexing service.
//
// The indexer consumes document-ingest events from Kafka, tokenises their
// content (with Porter stemming), and writes inverted-index entries into the
// appropriate shard. Each shard periodically flushes its in-memory index to
// immutable on-disk segments for durability.
//
// Usage:
//
//	go run ./cmd/indexer [-config configs/development.yaml]
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/consumer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/shard"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// numShards is the fixed number of index shards matching the shard count in
// the searcher service.
const numShards = 8

// main initialises the shard router, starts flush loops for every shard, then
// consumes Kafka messages until SIGINT/SIGTERM. Before exiting it flushes all
// shards one final time to ensure no data loss.
func main() {
	configPath := flag.String("config", "configs/development.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Logging.Level, cfg.Logging.Format)
	slog.Info("starting indexer service", "num_shards", numShards)
	router, err := shard.NewRouter(cfg.Indexer, numShards)
	if err != nil {
		slog.Error("failed to create shard router", "error", err)
		os.Exit(1)
	}
	defer router.Close()

	// PostgreSQL — used to update document status after indexing.
	db, err := postgres.New(cfg.Postgres)
	if err != nil {
		slog.Warn("postgres not available, document status will not be updated", "error", err)
	}
	if db != nil {
		defer db.Close()
		slog.Info("connected to postgres for status updates")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for shardID, engine := range router.GetAllEngines() {
		engine.StartFlushLoop(ctx)
		slog.Info("flush loop started", "shard_id", shardID)
	}
	var sqlDB *sql.DB
	if db != nil {
		sqlDB = db.DB
	}
	handler := consumer.HandleMessageSharded(router, sqlDB)
	kafkaConsumer := kafka.NewConsumer(
		cfg.Kafka,
		cfg.Kafka.Topics.DocumentIngest,
		handler,
	)

	indexConsumer := consumer.New(kafkaConsumer)

	slog.Info("indexer service ready, consuming from kafka",
		"topic", cfg.Kafka.Topics.DocumentIngest,
		"group", cfg.Kafka.ConsumerGroup,
	)

	if err := indexConsumer.Start(ctx); err != nil {
		slog.Error("consumer error", "error", err)
	}

	slog.Info("flushing all shards before shutdown")
	if err := router.FlushAll(); err != nil {
		slog.Error("final flush failed", "error", err)
	}

	slog.Info("indexer service stopped")
}
