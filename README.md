# Distributed Search & Analytics Platform

A production-grade distributed full-text search engine built from scratch in Go. Features custom inverted indexing, BM25 ranking, distributed sharding, query caching, an API gateway with authentication, analytics pipelines, and full observability — no Elasticsearch, no Lucene, no external search libraries.

## Architecture

```
                              ┌───────────────┐
                              │   API Gateway │  :8082
                              │  (Auth + Rate │
                              │   Limiting)   │
                              └──────┬────────┘
                        ┌────────────┼────────────┐
                        ▼            │             ▼
               ┌─────────────┐      │      ┌─────────────┐
               │  Ingestion  │      │      │   Search    │  :8080
               │   Service   │ :8081│      │   Service   │
               └──────┬──────┘      │      └──────┬──────┘
                      │             │             │
                      ▼             │             ▼
               ┌─────────────┐      │      ┌──────────────┐
               │    Kafka    │      │      │   Sharded    │
               │             │──────┘      │  Executor    │
               └──────┬──────┘             └──────┬───────┘
                      │                           │
           ┌─────────┴─────────┐           ┌──────┴───────┐
           ▼                   ▼           │  8 Shards ×  │
    ┌─────────────┐     ┌──────────┐       │  Inverted    │
    │   Indexer   │     │Analytics │       │  Index       │
    │  (8 shards) │     │ Pipeline │       └──────────────┘
    └──────┬──────┘     └──────────┘              │
           │                                      ▼
    ┌──────┴───────┐                       ┌─────────────┐
    │   Segment    │                       │ Redis Cache │
    │   Storage    │                       └─────────────┘
    └──────────────┘
                              ┌──────────────┐
                              │  PostgreSQL  │  Metadata
                              └──────────────┘
                              ┌──────────────┐
                              │  Prometheus  │  :9090
                              └──────────────┘
```

## Features

- **Custom Inverted Index** — LSM-tree-inspired with immutable segments, binary search dictionary, and atomic flush-to-disk
- **BM25 Ranking** — Full Okapi BM25 with global IDF across shards, configurable k1/b parameters
- **Distributed Sharding** — 8 shards with consistent hash assignment, parallel fan-out queries
- **API Gateway** — Unified entry point with authentication, rate limiting, CORS, and request routing
- **API Key Authentication** — SHA-256 hashed keys stored in PostgreSQL with per-key rate limits and expiry
- **Rate Limiting** — Token-bucket rate limiter scoped per API key
- **Query Caching** — Redis-backed with singleflight stampede prevention and SHA-256 cache keys
- **Boolean Queries** — AND, OR, NOT operators with stemming and stop word removal
- **Analytics Pipeline** — Kafka-based event streaming with real-time aggregation, percentile tracking, and persistent snapshots
- **Observability** — Prometheus RED metrics, structured tracing with span hierarchy, health checks
- **Resilience** — Circuit breakers, exponential backoff retry with jitter, request timeouts
- **RPC Framework** — Lightweight JSON-over-TCP RPC for internal service communication
- **OpenAPI Spec** — Full OpenAPI 3.0.3 specification for the entire API surface
- **Zero Dependencies at Runtime** — Scratch-based Docker images (~15MB)

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start — Docker Compose](#quick-start--docker-compose)
- [Quick Start — Run Locally](#quick-start--run-locally)
- [Services](#services)
- [API Usage](#api-usage)
- [API Key Management](#api-key-management)
- [API Endpoints](#api-endpoints)
- [Project Structure](#project-structure)
- [Configuration](#configuration)
- [Environment Variables](#environment-variables)
- [Building](#building)
- [Testing](#testing)
- [Benchmarks](#benchmarks)
- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [License](#license)

---

## Prerequisites

| Tool | Version | Required For |
|------|---------|-------------|
| **Go** | 1.25+ | Building and running services |
| **Docker** | 20.10+ | Container images |
| **Docker Compose** | v2+ | Local development stack |
| **PostgreSQL** | 16+ | Metadata store (included in Docker Compose) |
| **Kafka** | 3.5+ | Message broker (included in Docker Compose) |
| **Redis** | 7+ | Query cache (included in Docker Compose) |
| **kubectl** | 1.27+ | Kubernetes deployment (optional) |
| **golangci-lint** | 1.55+ | Linting (optional) |

---

## Quick Start — Docker Compose

The fastest way to get everything running. This starts Zookeeper, Kafka, Redis, PostgreSQL, and all application services:

```bash
# Clone the repository
git clone https://github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform.git
cd Distributed-Search-Analytics-Platform

# Start the full stack
docker compose up --build -d
```

Verify all containers are healthy:

```bash
docker compose ps
```

Expected output — all services should show `running (healthy)` or `running`:

| Container | Port | Description |
|-----------|------|-------------|
| sp-postgres | 5432 | PostgreSQL (auto-runs migrations on first start) |
| sp-kafka | 9092 | Kafka message broker |
| sp-redis | 6379 | Redis cache |
| sp-zookeeper | 2181 | Kafka coordination |
| sp-ingestion | 8081 | Document ingestion API |
| sp-indexer | — | Kafka consumer → index builder |
| sp-searcher | 8080 | Search API + metrics on 9090 |

**Stop everything:**

```bash
docker compose down           # Stop and remove containers
docker compose down -v        # Also remove data volumes (clean reset)
```

**View logs:**

```bash
docker compose logs -f                  # All services
docker compose logs -f ingestion        # Single service
docker compose logs --tail 50 searcher  # Last 50 lines
```

---

## Quick Start — Run Locally

For local development without Docker for the Go services (infrastructure still runs in Docker).

### Step 1: Start Infrastructure

```bash
# PostgreSQL
docker run -d --name sp-postgres \
  -p 5432:5432 \
  -e POSTGRES_DB=searchplatform \
  -e POSTGRES_USER=searchplatform \
  -e POSTGRES_PASSWORD=localdev \
  -v $(pwd)/migrations/postgres/001_initial_schema.up.sql:/docker-entrypoint-initdb.d/001_schema.sql \
  postgres:16-alpine

# Zookeeper
docker run -d --name sp-zookeeper \
  -p 2181:2181 \
  -e ZOOKEEPER_CLIENT_PORT=2181 \
  confluentinc/cp-zookeeper:7.5.0

# Kafka (wait ~5s for Zookeeper to be ready)
docker run -d --name sp-kafka \
  -p 9092:9092 \
  --link sp-zookeeper \
  -e KAFKA_BROKER_ID=1 \
  -e KAFKA_ZOOKEEPER_CONNECT=sp-zookeeper:2181 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
  -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  confluentinc/cp-kafka:7.5.0

# Redis
docker run -d --name sp-redis -p 6379:6379 redis:7-alpine
```

### Step 2: Start the Core Services

Open **three separate terminals** and run each service:

**Terminal 1 — Ingestion Service** (receives documents, publishes to Kafka):
```bash
go run ./cmd/ingestion
# Listens on :8081 by default
```

**Terminal 2 — Indexer** (consumes from Kafka, builds inverted index shards):
```bash
go run ./cmd/indexer
# No HTTP port — purely a Kafka consumer
```

**Terminal 3 — Search Service** (query engine + cache + analytics + metrics):
```bash
go run ./cmd/searcher
# Search API on :8080, Prometheus metrics on :9090
```

### Step 3 (Optional): Start the Gateway

The gateway provides a unified API with authentication and rate limiting:

```bash
go run ./cmd/gateway
# Listens on :8082, proxies to ingestion (:8081) and search (:8080)
```

### Step 4 (Optional): Start the Analytics Service

Standalone analytics aggregation from Kafka events:

```bash
go run ./cmd/analytics
# Listens on :8080 (uses server.port from config)
```

> **Tip:** Override ports with environment variables:  
> `SP_SERVER_PORT=8083 go run ./cmd/analytics`

---

## Services

| Service | Command | Default Port | Description |
|---------|---------|-------------|-------------|
| **Ingestion** | `go run ./cmd/ingestion` | 8081 | HTTP API accepting documents, validates, stores metadata in PostgreSQL, publishes to Kafka |
| **Indexer** | `go run ./cmd/indexer` | — (no HTTP) | Kafka consumer that tokenizes documents and builds inverted index across 8 shards |
| **Searcher** | `go run ./cmd/searcher` | 8080 | Full-text search with BM25 ranking, Redis cache, analytics tracking |
| **Gateway** | `go run ./cmd/gateway` | 8082 | API gateway — auth, rate limiting, CORS, proxies to ingestion and search |
| **Analytics** | `go run ./cmd/analytics` | 8080 | Standalone analytics aggregation from Kafka events |
| **Auth CLI** | `go run ./cmd/auth` | — (CLI) | Command-line tool for managing API keys |
| **Load Test** | `go run ./cmd/loadtest` | — (CLI) | HTTP load testing tool targeting the search service |

---

## API Usage

### Ingest a Document (Direct)

```bash
curl -X POST http://localhost:8081/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Distributed Systems",
    "body": "A guide to building distributed search engines with Go."
  }'
```

### Ingest via Gateway (Authenticated)

```bash
# First create an API key (see API Key Management below)
curl -X POST http://localhost:8082/api/v1/documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-api-key>" \
  -d '{
    "title": "Distributed Systems",
    "body": "A guide to building distributed search engines with Go."
  }'
```

### Search

```bash
# Direct
curl "http://localhost:8080/api/v1/search?q=distributed+search&limit=10"

# Via gateway
curl "http://localhost:8082/api/v1/search?q=distributed+search&limit=10" \
  -H "Authorization: Bearer <your-api-key>"
```

### Boolean Queries

```bash
curl "http://localhost:8080/api/v1/search?q=distributed+AND+search+NOT+monolithic"
```

### Cache Operations

```bash
# View cache stats
curl http://localhost:8080/api/v1/cache/stats

# Invalidate cache
curl -X POST http://localhost:8080/api/v1/cache/invalidate
```

### Analytics

```bash
curl http://localhost:8080/api/v1/analytics
```

### Health Checks

```bash
curl http://localhost:8080/health/live    # Liveness
curl http://localhost:8080/health/ready   # Readiness (checks all deps)
curl http://localhost:8081/health          # Ingestion health
```

---

## API Key Management

The `auth` CLI tool manages API keys stored in PostgreSQL.

### Create an API Key

```bash
go run ./cmd/auth create --name "my-app" --rate-limit 100 --expires-in 720h
```

Output:
```
API key created successfully.
Store this key securely — it cannot be retrieved again.

  Key:        <raw-key-string>
  Name:       my-app
  Rate Limit: 100 req/min
  Expires:    2026-03-16T10:00:00Z
```

### List All Active Keys

```bash
go run ./cmd/auth list
```

### Revoke a Key

```bash
go run ./cmd/auth -- revoke --key "<raw-key-string>"
```

### Using Keys with the Gateway

Pass the key in any of these ways:

```bash
# Authorization header (recommended)
curl -H "Authorization: Bearer <key>" http://localhost:8082/api/v1/search?q=test

# X-API-Key header
curl -H "X-API-Key: <key>" http://localhost:8082/api/v1/search?q=test

# Query parameter
curl "http://localhost:8082/api/v1/search?q=test&api_key=<key>"
```

---

## API Endpoints

### Ingestion Service (`:8081`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/documents` | Ingest a document |
| GET | `/health` | Health check |

### Search Service (`:8080`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/search?q=<query>&limit=<n>` | Full-text search with BM25 ranking |
| GET | `/api/v1/cache/stats` | Cache hit/miss statistics |
| POST | `/api/v1/cache/invalidate` | Clear the search cache |
| GET | `/api/v1/analytics` | Search analytics (query counts, latencies, top queries) |
| GET | `/health/live` | Liveness probe |
| GET | `/health/ready` | Readiness probe (checks all dependencies) |

### Gateway (`:8082`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/documents` | Yes | Proxy to ingestion service |
| GET | `/api/v1/search` | Yes | Proxy to search service |
| GET | `/api/v1/documents/:id` | Yes | Get document by ID (direct DB) |
| GET | `/api/v1/documents` | Yes | List documents (direct DB) |
| GET | `/api/v1/analytics` | Yes | Proxy to search analytics |
| GET | `/api/v1/cache/stats` | Yes | Proxy to cache stats |
| POST | `/admin/api-keys` | Yes | Create a new API key |
| GET | `/admin/api-keys` | Yes | List all API keys |
| GET | `/health/live` | No | Liveness probe |
| GET | `/health/ready` | No | Readiness probe |

### Metrics (`:9090`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/metrics` | Prometheus metrics |

> **Full API spec:** See [api/openapi/openapi.yaml](api/openapi/openapi.yaml) for the complete OpenAPI 3.0.3 specification.

---

## Project Structure

```
├── api/
│   ├── openapi/                # OpenAPI 3.0.3 specification
│   │   └── openapi.yaml
│   └── proto/                  # Protocol Buffer definitions
│       ├── common/common.proto # Shared types (Document, Pagination)
│       ├── search/search.proto # Search service definition
│       └── index/index.proto   # Index service definition
├── cmd/                        # Service entry points
│   ├── analytics/              # Standalone analytics service
│   ├── auth/                   # CLI for API key management
│   ├── gateway/                # API gateway service
│   ├── ingestion/              # Document ingestion API
│   ├── indexer/                # Kafka consumer → index builder
│   ├── searcher/               # Search API + analytics + metrics
│   └── loadtest/               # HTTP load testing tool
├── configs/                    # Environment-specific YAML configs
│   ├── development.yaml
│   ├── staging.yaml
│   └── production.yaml
├── data/index/                 # Runtime index data (shard-0 through shard-7)
├── deployments/
│   ├── docker/                 # Multi-stage Dockerfiles
│   │   ├── Dockerfile.ingestion
│   │   ├── Dockerfile.indexer
│   │   └── Dockerfile.searcher
│   └── k8s/                    # Kubernetes manifests (Kustomize)
│       ├── base/
│       └── overlays/
├── internal/
│   ├── analytics/              # Event collection + aggregation
│   │   ├── aggregator/store.go # PostgreSQL persistence for snapshots
│   │   └── collector/batch.go  # Batch event collector (size/time flush)
│   ├── auth/
│   │   ├── apikey/             # API key validator (SHA-256 + PostgreSQL)
│   │   └── ratelimit/          # Token-bucket rate limiter
│   ├── gateway/
│   │   ├── handler/            # Gateway HTTP handlers (proxy + direct DB)
│   │   ├── middleware/         # Auth, CORS, rate limit middleware
│   │   └── router/            # Route table + middleware chain
│   ├── ingestion/              # Validation, publishing, handlers
│   ├── indexer/
│   │   ├── tokenizer/          # Tokenization + Porter stemming
│   │   ├── index/              # In-memory inverted index
│   │   ├── segment/            # Immutable on-disk segments (read/write)
│   │   ├── shard/              # Multi-shard router
│   │   ├── consumer/           # Kafka consumer handler
│   │   └── engine.go           # Orchestrator (index + flush + search)
│   └── searcher/
│       ├── parser/             # Boolean query parser (AND/OR/NOT)
│       ├── ranker/             # BM25 scoring
│       ├── executor/           # Single + sharded query execution
│       ├── merger/             # Cross-shard result merging (min-heap)
│       ├── cache/              # Redis cache with singleflight
│       └── handler/            # HTTP search handler
├── migrations/                 # PostgreSQL schema migrations
│   └── postgres/
│       ├── 001_initial_schema.up.sql
│       └── 001_initial_schema.down.sql
├── pkg/                        # Shared libraries
│   ├── config/                 # YAML + env var configuration
│   ├── errors/                 # Sentinel errors + AppError
│   ├── grpc/                   # JSON-over-TCP RPC framework
│   │   ├── server.go           # RPC server with method registration
│   │   └── client.go           # RPC client with connection pooling
│   ├── health/                 # Health checker with component registry
│   ├── kafka/                  # Producer + consumer wrappers
│   ├── logger/                 # Structured logging (slog)
│   ├── metrics/                # Prometheus metrics + server
│   ├── middleware/             # HTTP middleware (timeout, request ID, metrics)
│   ├── postgres/               # PostgreSQL connection pool
│   ├── proto/                  # Go message types (mirrors .proto defs)
│   ├── redis/                  # Redis client wrapper
│   ├── resilience/             # Circuit breaker, retry, timeout
│   └── tracing/                # Structured span tracing
├── test/
│   ├── benchmark/              # Go micro-benchmarks
│   ├── e2e/                    # End-to-end platform tests
│   └── integration/            # Integration tests (gateway, etc.)
├── docker-compose.yml          # Full local development stack
├── Makefile                    # Build, test, lint, deploy commands
├── go.mod
└── go.sum
```

---

## Configuration

Configuration uses three layers (highest priority wins):

1. **Environment variables** (prefix `SP_`)
2. **YAML config file** (specified via `-config` flag)
3. **Built-in defaults**

All services accept the `-config` flag:

```bash
go run ./cmd/searcher -config configs/production.yaml
```

See [configs/development.yaml](configs/development.yaml) for all options.

### Key Configuration Sections

| Section | Description |
|---------|-------------|
| `server` | HTTP port, read/write timeouts, shutdown timeout |
| `postgres` | Host, port, credentials, connection pool settings |
| `kafka` | Broker addresses, consumer group, topic names |
| `redis` | Address, password, pool size, cache TTL |
| `indexer` | Data directory, segment size, flush/merge intervals |
| `search` | Max results, default limit, timeout per shard |
| `gateway` | Port, upstream URLs for ingestion and search |
| `logging` | Level (debug/info/warn/error), format (text/json) |
| `tracing` | Enable/disable, endpoint, sample rate |
| `metrics` | Enable/disable, Prometheus port |

---

## Environment Variables

All environment variables use the `SP_` prefix. These override YAML config values.

| Variable | Default | Description |
|----------|---------|-------------|
| `SP_SERVER_PORT` | `8080` | HTTP server port |
| `SP_POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `SP_POSTGRES_PORT` | `5432` | PostgreSQL port |
| `SP_POSTGRES_DATABASE` | `searchplatform` | Database name |
| `SP_POSTGRES_USER` | `searchplatform` | Database user |
| `SP_POSTGRES_PASSWORD` | `localdev` | Database password |
| `SP_KAFKA_BROKERS` | `localhost:9092` | Kafka broker addresses |
| `SP_REDIS_ADDR` | `localhost:6379` | Redis address |
| `SP_INDEXER_DATADIR` | `./data/index` | Index data directory |
| `SP_METRICS_PORT` | `9090` | Prometheus metrics port |
| `SP_LOG_LEVEL` | `info` | Log level |
| `SP_LOG_FORMAT` | `text` | Log format (text/json) |
| `SP_GATEWAY_PORT` | `8082` | Gateway HTTP port |
| `SP_GATEWAY_INGESTION_URL` | `http://localhost:8081` | Upstream ingestion URL |
| `SP_GATEWAY_SEARCHER_URL` | `http://localhost:8080` | Upstream search URL |

---

## Building

### Build All Services

```bash
make build
# Outputs binaries to ./bin/
```

### Build Individual Services

```bash
make gateway
make ingestion
make indexer
make searcher
make analytics
make auth
```

### Build with Go Directly

```bash
go build -o bin/gateway    ./cmd/gateway
go build -o bin/ingestion  ./cmd/ingestion
go build -o bin/indexer    ./cmd/indexer
go build -o bin/searcher   ./cmd/searcher
go build -o bin/analytics  ./cmd/analytics
go build -o bin/auth       ./cmd/auth
go build -o bin/loadtest   ./cmd/loadtest
```

### Build Docker Images

```bash
make docker-build
# Builds: searchplatform-ingestion, searchplatform-indexer, searchplatform-searcher
```

---

## Testing

### Unit Tests

```bash
make test
# Runs all tests with race detection + coverage report

make test-verbose
# Same but with verbose output
```

### Integration Tests

Requires running infrastructure (PostgreSQL, Kafka, Redis):

```bash
# Start infrastructure first
docker compose up -d postgres kafka redis zookeeper

# Run integration tests
make test-integration
# or
go test -race -tags=integration ./test/integration/...
```

### End-to-End Tests

Requires the full stack running:

```bash
# Start everything
docker compose up --build -d

# Run e2e tests
go test -race -tags=e2e ./test/e2e/...
```

### Linting

```bash
make lint
# Requires golangci-lint installed
```

### Format Code

```bash
make fmt
```

---

## Benchmarks

### Micro-Benchmarks

```bash
go test -bench="." -benchmem ./test/benchmark/...
```

### Load Testing

```bash
# Basic load test
go run ./cmd/loadtest -url http://localhost:8080 -concurrency 20 -duration 30s

# High concurrency
go run ./cmd/loadtest -url http://localhost:8080 -concurrency 100 -duration 60s
```

Sample results (local, 8 shards, 10K docs):
- **2,200+ req/sec** at 5 concurrent workers
- **P50: 2.1ms**, P99: 4.2ms
- **Tokenizer: ~35 MB/s** single-threaded, ~150 MB/s parallel

---

## Deployment

### Docker Compose (Development / Staging)

```bash
docker compose up --build -d    # Start everything
docker compose ps               # Check status
docker compose logs -f          # Follow all logs
docker compose down             # Stop everything
docker compose down -v          # Stop + remove volumes
```

### Kubernetes (Production)

```bash
# Dev environment
kubectl apply -k deployments/k8s/overlays/dev

# Production
kubectl apply -k deployments/k8s/overlays/prod

# Check status
kubectl get pods -l app=searchplatform
kubectl logs -f deployment/searcher
```

### Database Migrations

Migrations are auto-applied when PostgreSQL starts via Docker Compose (mounted as init script). For manual migration:

```bash
# Requires golang-migrate CLI
# Install: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Apply migrations
migrate -path migrations/postgres \
  -database "postgres://searchplatform:localdev@localhost:5432/searchplatform?sslmode=disable" up

# Roll back last migration
migrate -path migrations/postgres \
  -database "postgres://searchplatform:localdev@localhost:5432/searchplatform?sslmode=disable" down 1
```

---

## Monitoring

### Prometheus Metrics

Available at `:9090/metrics` when the search service is running.

Key metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Request count by method, path, status |
| `search_latency_seconds` | Histogram | Search latency by cache status |
| `search_queries_total` | Counter | Query count by result type |
| `cache_hits_total` | Counter | Cache hit count |
| `cache_misses_total` | Counter | Cache miss count |
| `active_shards` | Gauge | Number of healthy shards |

### Health Checks

```bash
# Liveness — is the service alive?
curl http://localhost:8080/health/live

# Readiness — are all dependencies connected?
curl http://localhost:8080/health/ready
```

### Common Debugging Commands

```bash
# Check if Kafka topics exist
docker exec sp-kafka kafka-topics --list --bootstrap-server localhost:9092

# Check PostgreSQL tables
docker exec -it sp-postgres psql -U searchplatform -c '\dt'

# Check Redis cache keys
docker exec sp-redis redis-cli keys '*'

# View document count
docker exec -it sp-postgres psql -U searchplatform -c 'SELECT count(*) FROM documents;'

# View API keys
docker exec -it sp-postgres psql -U searchplatform -c 'SELECT id, name, rate_limit, is_active, created_at FROM api_keys;'
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.