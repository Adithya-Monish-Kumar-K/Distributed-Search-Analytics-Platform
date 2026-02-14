# Architecture

## System Overview

The platform is composed of three independently deployable services connected via Apache Kafka:

### 1. Ingestion Service (`cmd/ingestion`)

Accepts documents via HTTP POST, validates them, assigns a shard (consistent hash on document ID), and publishes to Kafka's `document.ingest` topic.

**Key design decisions:**
- Returns `202 Accepted` immediately — the caller doesn't wait for indexing
- Idempotency check in the publisher prevents duplicate processing
- Shard assignment happens at ingestion time, not at indexing, ensuring deterministic routing

### 2. Indexer Service (`cmd/indexer`)

Consumes from Kafka and builds the inverted index. Each indexer instance manages 8 shards, each with its own engine:

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

Handles search queries with a multi-stage pipeline:

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
HTTP Response (JSON)
```

## Component Interaction

```
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
                             │
                      ┌──────┴───────┐
                      │   Segment    │
                      │   Storage    │
                      │  (8 shards)  │
                      └──────┬───────┘
                             │
     ┌─────────────┐  ┌──────┴───────┐  ┌──────────────┐
     │    Redis    │◀─│   Searcher   │──│  Prometheus  │
     │   Cache     │  │   Service   │  │   Metrics    │
     └─────────────┘  └──────────────┘  └──────────────┘
```

## Threading Model

- **Ingestion**: Single goroutine per HTTP request. Kafka publish is synchronous (guaranteed delivery).
- **Indexer**: Single Kafka consumer goroutine. Index writes are serialized per shard (mutex-protected). Flush loop runs in a separate goroutine per engine.
- **Searcher**: One goroutine per HTTP request. Sharded executor spawns N goroutines (one per shard) for parallel fan-out, synchronized via `sync.WaitGroup`.

## Data Storage

| Data | Storage | Durability |
|------|---------|------------|
| Documents (metadata) | PostgreSQL | Persistent, WAL-protected |
| Inverted index (memory) | In-process RAM | Lost on crash (rebuilt from Kafka) |
| Inverted index (segments) | Local filesystem | Persistent, atomic writes |
| Query cache | Redis | Ephemeral, TTL-based |
| Analytics events | Kafka → in-memory aggregation | Events are durable in Kafka |

## Failure Modes

| Failure | Impact | Recovery |
|---------|--------|----------|
| Redis down | Cache disabled, all queries go to index | Automatic reconnect, graceful degradation |
| Kafka down | No new documents indexed, no analytics | Backpressure on ingestion (returns 500) |
| Single shard engine crash | Partial results for that shard | Other 7 shards still respond |
| All shards fail | Search returns error | Restart required |
| PostgreSQL down | Ingestion fails (metadata store) | Auto-reconnect via connection pool |