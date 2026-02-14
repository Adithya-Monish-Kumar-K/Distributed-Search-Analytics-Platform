// Command searcher starts the distributed search service.
//
// The searcher loads shard data from disk, connects to Redis for query caching,
// starts an analytics collector/aggregator pipeline via Kafka, and exposes an
// HTTP API for full-text search, cache management, analytics, and health checks.
//
// Usage:
//
//	go run ./cmd/searcher [-config configs/development.yaml]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/analytics"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/shard"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/cache"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/executor"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/handler"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/health"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/metrics"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/middleware"
	pkgredis "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/redis"
)

// numShards is the fixed number of index shards. Each shard holds a subset of
// the indexed documents, determined by consistent hashing on document ID.
const numShards = 8

// main initialises all dependencies (config, logging, metrics, shard router,
// Redis cache, Kafka analytics pipeline, health checker) and starts the HTTP
// server on the configured port. Graceful shutdown is triggered by SIGINT/SIGTERM.
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
	var m *metrics.Metrics
	if cfg.Metrics.Enabled {
		m = metrics.New()
		metricsShutdown := metrics.StartServer(cfg.Metrics.Port)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
			defer cancel()
			metricsShutdown(shutdownCtx)
		}()
		m.ActiveShards.Set(float64(numShards))
		slog.Info("prometheus metrics enabled", "port", cfg.Metrics.Port)
	}
	router, err := shard.NewRouter(cfg.Indexer, numShards)
	if err != nil {
		slog.Error("failed to create shard router", "error", err)
		os.Exit(1)
	}
	defer router.Close()
	slog.Info("shard router initialized", "data_dir", cfg.Indexer.DataDir)

	if m != nil {
		for shardID, engine := range router.GetAllEngines() {
			m.ShardDocCount.WithLabelValues(strconv.Itoa(shardID)).Set(float64(engine.GetTotalDocs()))
		}
	}
	var queryCache *cache.QueryCache
	var redisClient *pkgredis.Client
	redisClient, err = pkgredis.NewClient(cfg.Redis)
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Periodically re-scan shard directories for segments flushed by the
	// indexer process so that newly ingested documents become searchable
	// without requiring a full restart.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n := router.ReloadAll(); n > 0 {
					slog.Info("hot-reloaded new segments", "count", n)
				}
			}
		}
	}()

	var collector *analytics.Collector
	analyticsProducer := kafka.NewProducer(cfg.Kafka, cfg.Kafka.Topics.AnalyticsEvents)
	collector = analytics.NewCollector(analyticsProducer, 10000)
	collector.Start(ctx)
	defer collector.Close()
	slog.Info("analytics collector started", "topic", cfg.Kafka.Topics.AnalyticsEvents)

	analyticsConsumer := kafka.NewConsumer(cfg.Kafka, cfg.Kafka.Topics.AnalyticsEvents, nil)
	aggregator := analytics.NewAggregator(analyticsConsumer)
	analyticsConsumer = kafka.NewConsumer(cfg.Kafka, cfg.Kafka.Topics.AnalyticsEvents, analytics.HandleEvent(aggregator))
	aggregator = analytics.NewAggregator(analyticsConsumer)
	analyticsH := analytics.NewHandler(aggregator)

	go func() {
		if err := aggregator.Start(ctx); err != nil {
			slog.Error("analytics aggregator error", "error", err)
		}
	}()
	slog.Info("analytics aggregator started")
	checker := health.NewChecker()
	checker.Register("index_engine", func(ctx context.Context) health.ComponentHealth {
		if router.NumShards() > 0 {
			return health.ComponentHealth{Status: health.StatusUp, Message: fmt.Sprintf("%d shards active", router.NumShards())}
		}
		return health.ComponentHealth{Status: health.StatusDown, Message: "no shards"}
	})
	checker.Register("redis", func(ctx context.Context) health.ComponentHealth {
		if redisClient == nil {
			return health.ComponentHealth{Status: health.StatusDegraded, Message: "not configured"}
		}
		if err := redisClient.Ping(ctx); err != nil {
			return health.ComponentHealth{Status: health.StatusDegraded, Message: err.Error()}
		}
		return health.ComponentHealth{Status: health.StatusUp}
	})
	exec := executor.NewSharded(router.GetAllEngines())
	h := handler.New(exec, queryCache, collector, m, cfg.Search.DefaultLimit, cfg.Search.MaxResults)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/search", h.Search)
	mux.HandleFunc("GET /api/v1/cache/stats", h.CacheStats)
	mux.HandleFunc("POST /api/v1/cache/invalidate", h.CacheInvalidate)
	mux.HandleFunc("GET /api/v1/analytics", analyticsH.Stats)
	mux.HandleFunc("GET /health/live", checker.LiveHandler())
	mux.HandleFunc("GET /health/ready", checker.ReadyHandler())
	var chain http.Handler = mux
	chain = middleware.Timeout(cfg.Server.WriteTimeout)(chain)
	if m != nil {
		chain = middleware.Metrics(m)(chain)
	}
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

	slog.Info("search service listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("search service stopped")
}
