# Distributed Search & Analytics Platform

A production-grade distributed full-text search engine built from scratch in Go. Features custom inverted indexing, BM25 ranking, distributed sharding, query caching, analytics pipelines, and full observability — no Elasticsearch, no Lucene, no external search libraries.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│  Ingestion  │────▶│    Kafka    │────▶│   Indexer    │
│   Service   │     │             │     │  (8 shards)  │
└─────────────┘     └──────┬──────┘     └──────┬───────┘
                           │                    │
                           ▼                    ▼
                    ┌─────────────┐     ┌──────────────┐
                    │  Analytics  │     │   Segment    │
                    │  Pipeline   │     │   Storage    │
                    └─────────────┘     └──────┬───────┘
                                               │
┌─────────────┐     ┌─────────────┐     ┌──────┴───────┐
│   Client    │────▶│   Search   │────▶│   Sharded    │
│             │     │   Handler   │     │  Executor    │
└─────────────┘     └──────┬──────┘     └──────────────┘
                           │
                    ┌──────┴──────┐
                    │ Redis Cache │
                    └─────────────┘
```

## Features

- **Custom Inverted Index** — LSM-tree-inspired with immutable segments, binary search dictionary, and atomic flush-to-disk
- **BM25 Ranking** — Full Okapi BM25 with global IDF across shards, configurable k1/b parameters
- **Distributed Sharding** — 8 shards with consistent hash assignment, parallel fan-out queries
- **Query Caching** — Redis-backed with singleflight stampede prevention and SHA-256 cache keys
- **Boolean Queries** — AND, OR, NOT operators with stemming and stop word removal
- **Analytics Pipeline** — Kafka-based event streaming with real-time aggregation and percentile tracking
- **Observability** — Prometheus RED metrics, structured tracing with span hierarchy, health checks
- **Resilience** — Circuit breakers, exponential backoff retry with jitter, request timeouts
- **Zero Dependencies at Runtime** — Scratch-based Docker images (~15MB)

## Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose

### Run with Docker Compose

```bash
docker compose up --build -d
```

This starts the full stack: Kafka, Redis, PostgreSQL, and all three services.

### Run Locally

Start infrastructure:
```bash
docker run -d --name zookeeper -p 2181:2181 -e ZOOKEEPER_CLIENT_PORT=2181 confluentinc/cp-zookeeper:7.5.0
docker run -d --name kafka -p 9092:9092 --link zookeeper -e KAFKA_BROKER_ID=1 -e KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181 -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 confluentinc/cp-kafka:7.5.0
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

Start services:
```bash
go run ./cmd/ingestion
go run ./cmd/indexer
go run ./cmd/searcher
```

### Ingest a Document

```bash
curl -X POST http://localhost:8081/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{"title": "Distributed Systems", "body": "A guide to building distributed search engines with Go."}'
```

### Search

```bash
curl "http://localhost:8080/api/v1/search?q=distributed+search&limit=10"
```

### Boolean Queries

```bash
curl "http://localhost:8080/api/v1/search?q=distributed+AND+search+NOT+monolithic"
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/documents` | Ingest a document |
| GET | `/api/v1/search?q=<query>&limit=<n>` | Full-text search |
| GET | `/api/v1/cache/stats` | Cache hit/miss statistics |
| POST | `/api/v1/cache/invalidate` | Clear search cache |
| GET | `/api/v1/analytics` | Search analytics dashboard |
| GET | `/health/live` | Liveness probe |
| GET | `/health/ready` | Readiness probe (checks all dependencies) |
| GET | `:9090/metrics` | Prometheus metrics |

## Project Structure

```
├── cmd/                    # Service entry points
│   ├── ingestion/          # Document ingestion API
│   ├── indexer/            # Kafka consumer → index builder
│   ├── searcher/           # Search API + analytics + metrics
│   └── loadtest/           # HTTP load testing tool
├── internal/
│   ├── ingestion/          # Validation, publishing, handlers
│   ├── indexer/
│   │   ├── tokenizer/      # Tokenization + Porter stemming
│   │   ├── index/          # In-memory inverted index
│   │   ├── segment/        # Immutable on-disk segments (read/write)
│   │   ├── shard/          # Multi-shard router
│   │   ├── consumer/       # Kafka consumer handler
│   │   └── engine.go       # Orchestrator (index + flush + search)
│   ├── searcher/
│   │   ├── parser/         # Boolean query parser (AND/OR/NOT)
│   │   ├── ranker/         # BM25 scoring
│   │   ├── executor/       # Single + sharded query execution
│   │   ├── merger/         # Cross-shard result merging (min-heap)
│   │   ├── cache/          # Redis cache with singleflight
│   │   └── handler/        # HTTP search handler
│   └── analytics/          # Event collection + aggregation
├── pkg/                    # Shared libraries
│   ├── config/             # YAML + env var configuration
│   ├── kafka/              # Producer + consumer wrappers
│   ├── redis/              # Redis client wrapper
│   ├── postgres/           # PostgreSQL connection pool
│   ├── logger/             # Structured logging (slog)
│   ├── errors/             # Sentinel errors + AppError
│   ├── metrics/            # Prometheus metrics + server
│   ├── tracing/            # Structured span tracing
│   ├── resilience/         # Circuit breaker, retry, timeout
│   ├── health/             # Health checker with component registry
│   └── middleware/         # HTTP middleware (timeout, request ID, metrics)
├── configs/                # Environment-specific YAML configs
├── deployments/
│   ├── docker/             # Multi-stage Dockerfiles
│   └── k8s/                # Kubernetes manifests (Kustomize)
├── migrations/             # PostgreSQL schema migrations
├── test/benchmark/         # Go benchmarks
├── docker-compose.yml      # Full local development stack
└── Makefile                # Build, test, lint, deploy commands
```

## Benchmarks

Run micro-benchmarks:
```bash
go test -bench="." -benchmem "./test/benchmark/..."
```

Run load test (requires running search service):
```bash
go run ./cmd/loadtest -url http://localhost:8080 -concurrency 20 -duration 30s
```

Sample results (local, 8 shards, 10K docs):
- **2,200+ req/sec** at 5 concurrent workers
- **P50: 2.1ms**, P99: 4.2ms
- **Tokenizer: ~35 MB/s** single-threaded, ~150 MB/s parallel

## Configuration

Configuration uses three layers (highest priority wins):
1. Environment variables (prefix `SP_`, e.g., `SP_SERVER_PORT=8080`)
2. YAML config file (specified via `-config` flag)
3. Defaults

See [configs/development.yaml](configs/development.yaml) for all options.

## Deployment

### Docker Compose
```bash
docker compose up --build -d    # Start everything
docker compose ps               # Check status
docker compose logs searcher    # View logs
docker compose down             # Stop everything
```

### Kubernetes
```bash
# Dev environment
kubectl apply -k deployments/k8s/overlays/dev

# Production
kubectl apply -k deployments/k8s/overlays/prod
```

## Monitoring

- **Prometheus metrics** at `:9090/metrics`
- **Health checks** at `/health/live` and `/health/ready`
- **Cache stats** at `/api/v1/cache/stats`
- **Analytics** at `/api/v1/analytics`

Key metrics to monitor:
- `http_requests_total` — Request rate by method/path/status
- `search_latency_seconds` — Search latency by cache status
- `search_queries_total` — Query count by result type
- `cache_hits_total` / `cache_misses_total` — Cache effectiveness
- `active_shards` — Number of healthy shards

## License

MIT License — see [LICENSE](LICENSE) for details.