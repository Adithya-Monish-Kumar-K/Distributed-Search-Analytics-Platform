package main

import (
	"context"
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
)

const numShards = 8

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for shardID, engine := range router.GetAllEngines() {
		engine.StartFlushLoop(ctx)
		slog.Info("flush loop started", "shard_id", shardID)
	}
	handler := consumer.HandleMessageSharded(router)
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
