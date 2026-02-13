package analytics

import "time"

type EventType string

const (
	EventSearch     EventType = "search"
	EventCacheHit   EventType = "cache_hit"
	EventCacheMiss  EventType = "cache_miss"
	EventIndexDoc   EventType = "index_document"
	EventZeroResult EventType = "zero_result"
)

type SearchEvent struct {
	Type       EventType `json:"type"`
	Query      string    `json:"query"`
	Terms      []string  `json:"terms"`
	TotalHits  int       `json:"total_hits"`
	Returned   int       `json:"returned"`
	LatencyMs  int64     `json:"latency_ms"`
	CacheHit   bool      `json:"cache_hit"`
	ShardCount int       `json:"shard_count"`
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"request_id"`
}

type IndexEvent struct {
	Type       EventType `json:"type"`
	DocumentID string    `json:"document_id"`
	ShardID    int       `json:"shard_id"`
	TokenCount int       `json:"token_count"`
	SizeBytes  int       `json:"size_bytes"`
	LatencyMs  int64     `json:"latency_ms"`
	Timestamp  time.Time `json:"timestamp"`
}
