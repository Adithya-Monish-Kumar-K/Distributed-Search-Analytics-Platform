# Architecture

## System Overview

The platform is composed of four independently deployable services connected via Apache Kafka, with an API Gateway providing unified access:

### 1. Ingestion Service (`cmd/ingestion`)

Accepts documents via HTTP POST, validates them, assigns a shard (consistent hash on document ID), stores metadata in PostgreSQL (status = `PENDING`), and publishes to Kafka's `document.ingest` topic.

**Key design decisions:**
- Returns `202 Accepted` immediately — the caller doesn't wait for indexing
- Idempotency check in the publisher prevents duplicate processing
- Shard assignment happens at ingestion time, not at indexing, ensuring deterministic routing
- Document metadata (title, content_size, shard_id, status) stored in PostgreSQL at ingest time

### 2. Indexer Service (`cmd/indexer`)

Consumes from Kafka and builds the inverted index. Each indexer instance manages 8 shards, each with its own engine. After successfully indexing a document, the indexer updates the document status in PostgreSQL from `PENDING` to `INDEXED` (or `FAILED` on error).

```
Kafka Consumer → Shard Router → Engine[0..7]
                                    │
                                    ├── MemoryIndex (in-memory inverted index)
                                    ├── SegmentWriter (atomic flush to disk)
                                    └── SegmentReader[] (immutable on-disk segments)
```

**Flush strategy:**
- Size-based: flushes when memory index exceeds `segmentMaxSize` (default 10MB)
- Time-based: periodic flush every `flushInterval` (default 5s)
- Shutdown: final flush on graceful shutdown

**Segment format:**
- Magic bytes: `0x53504458`
- Binary dictionary with sorted terms for binary search lookup
- JSON-encoded posting lists per term
- Atomic writes via temp file + rename (no partial segments on crash)

### 3. Searcher Service (`cmd/searcher`)

Handles search queries with a multi-stage pipeline. Includes a **periodic segment hot-reload** mechanism — every 10 seconds, the searcher scans each shard's data directory for new `.spdx` segment files and loads them automatically. This means newly indexed documents become searchable without any service restart.

```
HTTP Request
    │
    ▼
Query Parser (AND/OR/NOT → QueryPlan)
    │
    ▼
Cache Lookup (Redis + singleflight)
    │ miss
    ▼
Sharded Executor (parallel fan-out to 8 engines)
    │
    ▼
BM25 Ranker (global IDF across shards)
    │
    ▼
Result Merger (min-heap top-K)
    │
    ▼
HTTP Response (JSON envelope: query, total, took_ms, cache_hit, results)
```

### 4. API Gateway (`cmd/gateway`)

Unified entry point for all client-facing traffic. Handles cross-cutting concerns before proxying requests to upstream services:

- **Authentication** — Validates API keys via SHA-256 hash lookup in PostgreSQL (supports `Authorization: Bearer`, `X-API-Key` header, or `api_key` query parameter)
- **Rate Limiting** — Per-key token-bucket rate limiter
- **CORS** — Configurable cross-origin resource sharing
- **Request Routing** — Proxies to ingestion (`:8081`) and search (`:8080`) services
- **Direct DB Queries** — Serves document listing and API key management directly from PostgreSQL without proxying
- **API Key Management** — `POST/GET /api/v1/admin/keys` for creating and listing keys, `DELETE /api/v1/admin/keys/:id` for revocation

## Component Interaction

```
                              ┌───────────────┐
                              │  API Gateway  │  :8082
                              │ (Auth + Rate  │
                              │  Limiting)    │
                              └──────┬────────┘
                        ┌────────────┼────────────┐
                        ▼            │             ▼
                    ┌──────────────────────────────────────────┐
                    │              Kafka Cluster               │
                    │                                          │
                    │  document.ingest    analytics.events     │
                    └──────────┬──────────────┬────────────────┘
                               │              │
              ┌────────────────┤              │
              │                │              │
              ▼                ▼              ▼
     ┌─────────────┐  ┌──────────────┐  ┌──────────────┐
     │  Ingestion  │  │   Indexer    │  │  Analytics   │
     │   Service   │  │   Service   │  │  Aggregator  │
     └─────────────┘  └──────┬───────┘  └──────────────┘
            │                │
            │         ┌──────┴───────┐
            │         │   Segment    │
            │         │   Storage    │
            │         │  (8 shards)  │
            │         └──────┬───────┘
            │                │ hot-reload (10s)
            │                │
     ┌──────┴──────┐  ┌──────┴───────┐  ┌──────────────┐
     │ PostgreSQL  │◀─│   Searcher   │──│  Prometheus  │
     │  (metadata  │  │   Service   │  │   Metrics    │
     │   + status) │  └──────┬───────┘  └──────────────┘
     └─────────────┘         │
                      ┌──────┴───────┐
                      │    Redis    │
                      │   Cache     │
                      └─────────────┘
```

## Threading Model

- **Ingestion**: Single goroutine per HTTP request. Kafka publish is synchronous (guaranteed delivery). PostgreSQL metadata insert is also synchronous.
- **Indexer**: Single Kafka consumer goroutine. Index writes are serialized per shard (mutex-protected). Flush loop runs in a separate goroutine per engine. PostgreSQL status updates are synchronous after each document.
- **Searcher**: One goroutine per HTTP request. Sharded executor spawns N goroutines (one per shard) for parallel fan-out, synchronized via `sync.WaitGroup`. A background goroutine runs segment hot-reload every 10 seconds.
- **Gateway**: One goroutine per HTTP request. Middleware chain (auth → rate limit → CORS) runs synchronously before proxying or handling the request.

## Data Storage

| Data | Storage | Durability |
|------|---------|------------|
| Documents (metadata + status) | PostgreSQL | Persistent, WAL-protected |
| API keys (SHA-256 hashed) | PostgreSQL | Persistent, WAL-protected |
| Document status (PENDING/INDEXED/FAILED) | PostgreSQL | Updated by indexer after processing |
| Inverted index (memory) | In-process RAM | Lost on crash (rebuilt from Kafka) |
| Inverted index (segments) | Local filesystem | Persistent, atomic writes, hot-reloaded by searcher |
| Query cache | Redis | Ephemeral, TTL-based |
| Analytics events | Kafka → in-memory aggregation | Events are durable in Kafka |

## Failure Modes

| Failure | Impact | Recovery |
|---------|--------|----------|
| Redis down | Cache disabled, all queries go to index | Automatic reconnect, graceful degradation |
| Kafka down | No new documents indexed, no analytics | Backpressure on ingestion (returns 500) |
| Single shard engine crash | Partial results for that shard | Other 7 shards still respond |
| All shards fail | Search returns error | Restart required |
| PostgreSQL down | Ingestion fails (metadata store), gateway auth fails, indexer status updates fail | Auto-reconnect via connection pool |
| Gateway down | Clients lose authenticated entry point | Direct access to ingestion/searcher still works |
| Gateway auth failure | Request rejected with 401 | Verify API key is valid and not revoked |
| Segment hot-reload failure | New documents not searchable (up to 10s delay) | Next reload cycle retries automatically |
| Indexer status update failure | Documents stuck in PENDING | Indexing still succeeds; status can be fixed manually |