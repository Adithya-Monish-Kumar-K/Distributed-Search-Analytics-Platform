# ADR-001: Custom Inverted Index with Immutable Segments

## Status

Accepted

## Context

We needed a full-text search index that supports:
- Fast term lookups across millions of documents
- Concurrent reads during writes
- Crash recovery without data corruption
- Acceptable memory usage as corpus grows

Options considered:
1. **Use Elasticsearch/Solr** — Full-featured but defeats the project's purpose of building from scratch
2. **Use Bleve (Go search library)** — Good but hides the internals we want to understand
3. **Build custom inverted index** — Full control, deep learning experience

## Decision

Build a custom LSM-tree-inspired inverted index with:
- **In-memory index** (`MemoryIndex`) for active writes, protected by `sync.RWMutex`
- **Immutable on-disk segments** flushed atomically (temp file + rename)
- **Binary search dictionary** in segments for O(log n) term lookup
- **JSON posting lists** for simplicity (trade-off: larger than binary encoding)

## Consequences

**Positive:**
- Full understanding of how search indexing works
- Crash-safe: no partial segment writes (atomic rename)
- Lock-free reads on immutable segments
- Simple recovery: just re-read existing `.spdx` files on startup
- **Segment hot-reload**: searcher periodically scans for new segments and loads them without restart (10-second interval)

**Negative:**
- JSON posting lists are ~3x larger than binary encoding
- No segment compaction/merging (segments accumulate over time)
- No deletions (would require tombstones or segment rewrite)
- Single-machine storage (not distributed filesystem)

## Future Improvements

- Binary posting list encoding (varint + delta encoding)
- Segment merge compaction
- Document deletion via tombstone markers
- Distributed storage (S3, HDFS)

## Implemented Enhancements

- ✅ **Segment hot-reload** — Searcher's `Engine.ReloadSegments()` scans for new `.spdx` files every 10 seconds via a background goroutine, enabling zero-downtime document searchability
- ✅ **Document status tracking** — Indexer updates PostgreSQL with PENDING → INDEXED/FAILED status after processing each document
