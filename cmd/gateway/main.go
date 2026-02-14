// Command gateway starts the API gateway service.
//
// The gateway is the single entry point for external clients. It authenticates
// requests via API keys (SHA-256 validated against PostgreSQL), applies
// per-key rate limiting, and proxies requests to the ingestion and search
// services. It also exposes admin endpoints for API key management and a
// direct document-retrieval endpoint backed by PostgreSQL.
//
// Usage:
//
//	go run ./cmd/gateway [-config configs/development.yaml]
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
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/apikey"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/ratelimit"
	gwhandler "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/gateway/handler"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/gateway/router"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// main initialises PostgreSQL, the API-key validator, the rate limiter, the
// gateway handler + router middleware chain, and starts the HTTP server.
// Graceful shutdown is triggered by SIGINT/SIGTERM.
func main() {
	configPath := flag.String("config", "configs/development.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Logging.Level, cfg.Logging.Format)
	slog.Info("starting gateway service",
		"port", cfg.Gateway.Port,
		"ingestion_url", cfg.Gateway.IngestionURL,
		"searcher_url", cfg.Gateway.SearcherURL,
	)

	// PostgreSQL — shared with auth for API key validation + document retrieval.
	db, err := postgres.New(cfg.Postgres)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("connected to postgres")

	// Auth + rate limiting.
	validator := apikey.NewValidator(db)
	limiter := ratelimit.New(time.Minute)

	// Gateway handler → router with full middleware chain.
	h := gwhandler.New(gwhandler.Config{
		IngestionURL: cfg.Gateway.IngestionURL,
		SearcherURL:  cfg.Gateway.SearcherURL,
	}, db, validator)

	chain := router.New(h, validator, limiter)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Gateway.Port),
		Handler:      chain,
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

	slog.Info("gateway service listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("gateway service stopped")
}
