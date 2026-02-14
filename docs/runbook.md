# Operations Runbook

## Service Startup Order

1. Infrastructure: Zookeeper → Kafka → Redis → PostgreSQL
2. Application: Indexer → Ingestion → Searcher

The searcher can start before the indexer, but won't return results until indexes are built.

## Health Checks

```bash
# Liveness (is the process alive?)
curl http://localhost:8080/health/live
# Expected: {"status":"alive"}

# Readiness (are all dependencies healthy?)
curl http://localhost:8080/health/ready
# Expected: {"status":"up","components":{"index_engine":{"status":"up"},"redis":{"status":"up"}}}
```

## Common Issues

### Search returns zero results

**Cause 1: No documents indexed**
```bash
# Check shard doc counts via metrics
curl -s http://localhost:9090/metrics | grep shard_document_count
```
If all zeros, no documents have been ingested and indexed.

**Cause 2: Indexer not running**
Check if the indexer is consuming from Kafka:
```bash
docker logs sp-indexer --tail 50
```

**Cause 3: Query terms being stemmed away**
The query "running" becomes "run" after stemming. Check if the indexed documents contain the stemmed form.

### High search latency (> 100ms)

**Check 1: Cache miss rate**
```bash
curl http://localhost:8080/api/v1/cache/stats
```
If hit rate is below 50%, consider increasing `cacheTTL`.

**Check 2: Too many segments**
Each segment requires a disk read. Check segment count:
```bash
ls data/index/shard-*/  | grep -c ".spdx"
```
If > 50 segments per shard, the indexer needs segment compaction (not yet implemented).

**Check 3: GC pressure**
```bash
curl -s http://localhost:9090/metrics | grep go_gc_duration
```

### Redis connection refused

**Symptom:** Logs show `redis unavailable, search caching disabled`

**Resolution:**
1. Check Redis is running: `docker ps | grep redis`
2. Check connectivity: `redis-cli -h localhost ping`
3. Service gracefully degrades — search still works, just slower

### Kafka consumer lag

**Symptom:** Documents ingested but not appearing in search results

**Resolution:**
```bash
# Check consumer group lag
docker exec sp-kafka kafka-consumer-groups \
  --bootstrap-server localhost:9092 \
  --group searchplatform-dev \
  --describe
```

If lag is growing, the indexer isn't keeping up. Scale indexer instances or increase `segmentMaxSize` to reduce flush frequency.

### Circuit breaker open

**Symptom:** Metrics show `circuit_breaker_state` = 1

**Resolution:**
The circuit breaker opens after consecutive failures. It will automatically transition to half-open after the configured timeout and try a single request. If that succeeds, it closes. If it fails, it stays open.

Check the underlying service that's failing:
```bash
curl -s http://localhost:9090/metrics | grep circuit_breaker
```

## Monitoring Alerts (suggested thresholds)

| Metric | Warning | Critical |
|--------|---------|----------|
| `http_requests_total{status="5xx"}` rate | > 1/min | > 10/min |
| `search_latency_seconds` P99 | > 500ms | > 2s |
| `cache_misses_total` rate | > 80% miss rate | > 95% miss rate |
| `active_shards` | < 8 | < 4 |
| `http_requests_in_flight` | > 50 | > 100 |

## Graceful Shutdown

All services handle `SIGINT` and `SIGTERM`:
1. Stop accepting new requests
2. Drain in-flight requests (up to `shutdownTimeout`)
3. Flush memory indexes to disk (indexer/searcher)
4. Close Kafka consumers/producers
5. Close Redis and PostgreSQL connections

```bash
# Graceful stop
kill -SIGTERM <pid>

# Docker
docker compose stop    # graceful (sends SIGTERM)
docker compose down    # graceful stop + remove containers
```

## Backup & Recovery

**Index data:**
- Segments are immutable files in `data/index/shard-N/`
- Copy the entire `data/index/` directory for backup
- On recovery, the engine automatically loads all `.spdx` files on startup

**Redis cache:**
- Ephemeral by design — no backup needed
- Cache rebuilds automatically on cache misses

**PostgreSQL:**
```bash
# Backup
pg_dump -U searchplatform searchplatform > backup.sql

# Restore
psql -U searchplatform searchplatform < backup.sql
```
