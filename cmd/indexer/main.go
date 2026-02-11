package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/consumer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
)

func main() {
	configPath := flag.String("config", "configs/development.yaml", "path to config file")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	logger.Setup(cfg.Logging.Level, cfg.Logging.Format)
	slog.Info("starting indexer service")
	engine, err := indexer.NewEngine(cfg.Indexer)
	if err != nil {
		slog.Error("failed to create index engine", "error", err)
		os.Exit(1)
	}
	defer engine.Close()
	slog.Info("index engine initialized",
		"data_dir", cfg.Indexer.DataDir,
		"segment_max_size", cfg.Indexer.SegmentMaxSize,
	)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	engine.StartFlushLoop(ctx)
	handler := consumer.HandleMessage(engine)
	kafkaConsumer := kafka.NewConsumer(cfg.Kafka, cfg.Kafka.Topics.DocumentIngest, handler)
	ic := consumer.New(engine, kafkaConsumer)
	slog.Info("indexer service listening for events",
		"topic", cfg.Kafka.Topics.DocumentIngest,
		"consumer_group", cfg.Kafka.ConsumerGroup,
	)
	if err := ic.Start(ctx); err != nil {
		slog.Error("consumer error", "error", err)
		os.Exit(1)
	}
	slog.Info("indexer service stopped")
}
