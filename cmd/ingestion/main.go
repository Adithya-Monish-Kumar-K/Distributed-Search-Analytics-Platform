// Command ingestion starts the document ingestion HTTP service.
//
// The service accepts new documents via POST /api/v1/documents, validates them,
// persists metadata to PostgreSQL, and publishes them to a Kafka topic for
// downstream indexing. It provides a health endpoint at GET /health.
//
// Usage:
//
//	go run ./cmd/ingestion [-config configs/development.yaml]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/ingestion/handler"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/ingestion/publisher"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// main loads configuration, connects to PostgreSQL, creates the Kafka producer,
// wires up the ingestion handler, and starts the HTTP server. Graceful shutdown
// is triggered by SIGINT/SIGTERM.
func main() {
	configPath := flag.String("config", "configs/development.yaml", "path to config file")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	logger.Setup(cfg.Logging.Level, cfg.Logging.Format)
	slog.Info("starting ingestion service", "port", cfg.Server.Port)
	db, err := postgres.New(cfg.Postgres)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("connected to postgres")
	producer := kafka.NewProducer(cfg.Kafka, cfg.Kafka.Topics.DocumentIngest)
	defer producer.Close()
	slog.Info("kafka producer initialized", "topic", cfg.Kafka.Topics.DocumentIngest)
	pub := publisher.New(db, producer)
	h := handler.New(pub)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/documents", h.Ingest)
	mux.HandleFunc("GET /health", h.Health)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()
	slog.Info("ingestion service listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("ingestion service stopped")
}
