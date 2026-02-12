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

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/shard"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/cache"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/executor"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/handler"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	pkgredis "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/redis"
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
	slog.Info("starting search service", "port", cfg.Server.Port, "num_shards", numShards)
	router, err := shard.NewRouter(cfg.Indexer, numShards)
	if err != nil {
		slog.Error("failed to create shard router", "error", err)
		os.Exit(1)
	}
	defer router.Close()
	slog.Info("shard router initialized", "data_dir", cfg.Indexer.DataDir)
	var queryCache *cache.QueryCache
	redisClient, err := pkgredis.NewClient(cfg.Redis)
	if err != nil {
		slog.Warn("redis unavailable, search caching disabled", "error", err)
	} else {
		defer redisClient.Close()
		queryCache = cache.New(redisClient, cfg.Redis)
		slog.Info("search cache enabled",
			"addr", cfg.Redis.Addr,
			"ttl", cfg.Redis.CacheTTL,
		)
	}
	exec := executor.NewSharded(router.GetAllEngines())
	h := handler.New(exec, queryCache, cfg.Search.DefaultLimit, cfg.Search.MaxResults)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/search", h.Search)
	mux.HandleFunc("GET /api/v1/cache/stats", h.CacheStats)
	mux.HandleFunc("POST /api/v1/cache/invalidate", h.CacheInvalidate)
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

	slog.Info("search service listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("search service stopped")
}
