# Data Flow

## Document Ingestion Flow

```
Client POST /api/v1/documents
    │
    ▼
┌──────────────────────────────────────────────┐
│ Ingestion Handler                            │
│  1. Parse JSON body                          │
│  2. Validate fields (title, body required)   │
│  3. Generate document ID (UUID)              │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Publisher                                    │
│  1. Idempotency check (skip if already seen) │
│  2. Assign shard: hash(docID) % 8           │
│  3. Store metadata in PostgreSQL:            │
│     {docID, title, content_size, shard_id,   │
│      status=PENDING, created_at}             │
│  4. Build IngestEvent {docID, title, body,   │
│     shard, timestamp}                         │
│  5. Publish to Kafka topic: document.ingest  │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
           202 Accepted {document_id: "..."}
```

## Indexing Flow

```
Kafka: document.ingest
    │
    ▼
┌──────────────────────────────────────────────┐
│ Consumer                                     │
│  1. Decode JSON message → IngestEvent        │
│  2. Route to shard engine via ShardRouter    │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Engine.IndexDocument(docID, title, body)     │
│  1. Tokenize: lowercase → split → remove    │
│     stop words → Porter stem                 │
│  2. Build term→{docID, freq, positions} map  │
│  3. Add to MemoryIndex (RWMutex-protected)   │
│  4. Track doc length + total docs            │
│  5. If memIndex.Size() >= segmentMaxSize:    │
│     → Flush()                                │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Update Document Status in PostgreSQL         │
│  On success: status = INDEXED                │
│  On error:   status = FAILED                 │
└──────────────────┬───────────────────────────┘
                   │ (on flush)
                   ▼
┌──────────────────────────────────────────────┐
│ SegmentWriter.Write(snapshot)                │
│  1. Snapshot MemoryIndex (sorted terms)      │
│  2. Write temp file with magic + header      │
│  3. Write dictionary (term offsets)          │
│  4. Write posting lists (JSON per term)      │
│  5. Atomic rename: tmp → seg_<timestamp>.spdx│
│  6. Open SegmentReader for new file          │
│  7. Reset MemoryIndex                        │
└──────────────────────────────────────────────┘
```

## Search Flow

```
Client GET /api/v1/search?q=distributed+AND+search&limit=10
    │
    ▼
┌──────────────────────────────────────────────┐
│ Search Handler                               │
│  1. Extract query + limit from URL params    │
│  2. Start tracing span                       │
│  3. Parse query → QueryPlan                  │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Query Parser                                 │
│  "distributed AND search" →                  │
│  QueryPlan{                                  │
│    Terms: ["distribut", "search"],           │
│    Type: QueryAND,                           │
│    ExcludeTerms: [],                         │
│  }                                           │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Cache Lookup (Redis + singleflight)          │
│  Key: SHA-256("AND:distribut,search:10")     │
│  HIT → return cached result                 │
│  MISS → execute query, cache result          │
└──────────────────┬───────────────────────────┘
                   │ miss
                   ▼
┌──────────────────────────────────────────────┐
│ Sharded Executor (parallel fan-out)          │
│  for each shard [0..7]:                      │
│    goroutine → engine.Search("distribut")    │
│             → engine.Search("search")        │
│    collect: postings, totalDocs, avgDocLen   │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Result Merging + BM25 Ranking                │
│  1. Merge posting lists across shards        │
│  2. Intersect (AND) or union (OR) doc sets   │
│  3. Remove NOT-excluded documents            │
│  4. Compute global IDF: log((N-df)/df + 1)   │
│  5. Compute TF normalization per doc:        │
│     tf*(k1+1) / (tf + k1*(1-b+b*dl/avgdl))  │
│  6. Sort by score (descending), take top K   │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Response (JSON envelope)                     │
│  {                                           │
│    "query": "distributed AND search",        │
│    "total": 1250,                            │
│    "took_ms": 12,                            │
│    "cache_hit": false,                       │
│    "results": [                              │
│      {"doc_id": "abc-123", "score": 4.8721}, │
│      {"doc_id": "def-456", "score": 4.1203}  │
│    ]                                         │
│  }                                           │
└──────────────────────────────────────────────┘
```

## Analytics Flow

```
Search Handler (after response)
    │
    ▼
┌──────────────────────────────────────────────┐
│ Analytics Collector                          │
│  1. Non-blocking Track() via buffered chan   │
│  2. Background goroutine batches events      │
│  3. Publishes to Kafka: analytics.events     │
└──────────────────┬───────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────┐
│ Analytics Aggregator                         │
│  1. Consumes from Kafka                      │
│  2. Atomic counters: total queries, cache    │
│     hits/misses                              │
│  3. Latency tracking: p50, p95, p99         │
│  4. Top queries by frequency                 │
│  5. Exposes via GET /api/v1/analytics        │
└──────────────────────────────────────────────┘
```

## Gateway Request Flow

```
Client Request (with API key)
    │
    ▼
┌──────────────────────────────────────────────┐
│ Gateway Middleware Chain                      │
│  1. CORS headers                             │
│  2. Extract API key from:                    │
│     - Authorization: Bearer <key>            │
│     - X-API-Key: <key>                       │
│     - ?api_key=<key> query parameter         │
│  3. Validate key: SHA-256 → PostgreSQL lookup │
│  4. Rate limit check (token bucket per key)  │
└──────────────────┬───────────────────────────┘
                   │
          ┌────────┴─────────┐
          ▼                  ▼
   ┌────────────┐     ┌────────────┐
   │   Proxy    │     │  Direct DB │
   │  Upstream  │     │   Handler  │
   ├────────────┤     ├────────────┤
   │ /documents │→8081│ /documents │ (GET list)
   │ /search    │→8080│ /admin/keys│ (CRUD)
   │ /analytics │→8080│            │
   │ /cache/*   │→8080│            │
   └────────────┘     └────────────┘
```

## Segment Hot-Reload Flow

```
Searcher Background Goroutine (every 10 seconds)
    │
    ▼
┌──────────────────────────────────────────────┐
│ Engine.ReloadSegments()                      │
│  for each shard [0..7]:                      │
│    1. Scan data/index/shard-N/ for .spdx     │
│    2. Compare against already-loaded segments │
│    3. For each NEW segment file:             │
│       → Open SegmentReader                   │
│       → Append to engine's segment list      │
│    4. Log newly loaded segments              │
└──────────────────────────────────────────────┘

Timeline:
  t=0s   Document ingested → Kafka → Indexer
  t=1-5s Indexer builds index, flushes segment to disk
  t=5s   Indexer updates PostgreSQL: status = INDEXED
  t≤15s  Searcher hot-reload picks up new segment
  t≤15s  Document now searchable (no restart needed)
```