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

	"github.com/Adithya-Monish-Kumar-K/searchplatform/internal/ingestion/handler"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/internal/ingestion/publisher"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/pkg/kafka"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/pkg/postgres"
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
	slog.Info("kafka producer intialized", "topic", cfg.Kafka.Topics.DocumentIngest)
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
