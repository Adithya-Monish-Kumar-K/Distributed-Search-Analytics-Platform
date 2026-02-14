// Command analytics starts the standalone analytics aggregation service.
//
// It consumes search-analytics events from Kafka, aggregates them in memory
// (total queries, latency percentiles, cache hit rate, error rate, top queries),
// and exposes an HTTP API at GET /api/v1/analytics for dashboards.
//
// Usage:
//
//	go run ./cmd/analytics [-config configs/development.yaml]
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

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/analytics"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/health"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/middleware"
)

// main boots the standalone analytics service: it creates a Kafka consumer for
// analytics events, starts the in-memory aggregator, registers a health checker,
// and serves the HTTP API. Graceful shutdown is triggered by SIGINT/SIGTERM.
func main() {
	configPath := flag.String("config", "configs/development.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Logging.Level, cfg.Logging.Format)
	slog.Info("starting analytics service", "port", cfg.Server.Port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Kafka consumer for analytics events.
	consumer := kafka.NewConsumer(cfg.Kafka, cfg.Kafka.Topics.AnalyticsEvents, nil)
	aggregator := analytics.NewAggregator(consumer)

	// Re-create consumer with the actual handler now that aggregator exists.
	consumer = kafka.NewConsumer(cfg.Kafka, cfg.Kafka.Topics.AnalyticsEvents, analytics.HandleEvent(aggregator))
	aggregator = analytics.NewAggregator(consumer)

	go func() {
		if err := aggregator.Start(ctx); err != nil {
			slog.Error("aggregator error", "error", err)
		}
	}()
	slog.Info("analytics aggregator started", "topic", cfg.Kafka.Topics.AnalyticsEvents)

	// HTTP API.
	analyticsHandler := analytics.NewHandler(aggregator)

	checker := health.NewChecker()
	checker.Register("kafka", func(ctx context.Context) health.ComponentHealth {
		return health.ComponentHealth{Status: health.StatusUp, Message: "consumer active"}
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/analytics", analyticsHandler.Stats)
	mux.HandleFunc("GET /health/live", checker.LiveHandler())
	mux.HandleFunc("GET /health/ready", checker.ReadyHandler())

	var chain http.Handler = mux
	chain = middleware.RequestID(chain)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      chain,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	slog.Info("analytics service listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("analytics service stopped")
}
