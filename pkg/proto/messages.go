// Package proto defines the shared message types used for internal RPC
// communication between services in the Distributed Search & Analytics Platform.
//
// These types mirror the Protocol Buffer definitions in api/proto/ and are
// hand-written for zero-dependency usage. To regenerate from .proto files:
//
//	protoc --go_out=. --go-grpc_out=. api/proto/**/*.proto
//
// The hand-written types use JSON struct tags for serialization over the
// platform's lightweight JSON-over-TCP RPC layer (see pkg/grpc).
package proto

// ---------- Common ----------

// Document represents a document across all services.
type Document struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	ContentHash string `json:"content_hash"`
	ContentSize int32  `json:"content_size"`
	ShardID     int32  `json:"shard_id"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	IndexedAt   int64  `json:"indexed_at,omitempty"`
}

// Pagination controls limit/offset for list endpoints.
type Pagination struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

// HealthCheckResponse mirrors the gRPC health check spec.
type HealthCheckResponse struct {
	Status string `json:"status"` // SERVING, NOT_SERVING, UNKNOWN
}

// ---------- Search ----------

// SearchRequest is the input to the Search RPC.
type SearchRequest struct {
	Query string `json:"query"`
	Limit int32  `json:"limit"`
}

// SearchResponse is the output of the Search RPC.
type SearchResponse struct {
	Query     string         `json:"query"`
	TotalHits int32          `json:"total_hits"`
	Results   []SearchResult `json:"results"`
	LatencyMs int64          `json:"latency_ms"`
}

// SearchResult is a single scored document in the result set.
type SearchResult struct {
	DocID string  `json:"doc_id"`
	Title string  `json:"title"`
	Score float32 `json:"score"`
}

// SuggestRequest is the input to the Suggest RPC.
type SuggestRequest struct {
	Prefix   string `json:"prefix"`
	MaxItems int32  `json:"max_items"`
}

// SuggestResponse is the output of the Suggest RPC.
type SuggestResponse struct {
	Suggestions []string `json:"suggestions"`
}

// ---------- Index ----------

// IndexRequest is the input to the IndexDocument RPC.
type IndexRequest struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	ShardID    int32  `json:"shard_id"`
}

// IndexResponse is the output of the IndexDocument RPC.
type IndexResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// StatsRequest optionally filters by shard (0 = all).
type StatsRequest struct {
	ShardID int32 `json:"shard_id"`
}

// StatsResponse contains index-level statistics.
type StatsResponse struct {
	TotalDocs      int64       `json:"total_docs"`
	TotalSegments  int64       `json:"total_segments"`
	TotalSizeBytes int64       `json:"total_size_bytes"`
	Shards         []ShardStat `json:"shards,omitempty"`
}

// ShardStat holds per-shard statistics.
type ShardStat struct {
	ShardID      int32 `json:"shard_id"`
	DocCount     int64 `json:"doc_count"`
	SegmentCount int64 `json:"segment_count"`
	SizeBytes    int64 `json:"size_bytes"`
}

// FlushRequest triggers a segment flush.
type FlushRequest struct {
	ShardID int32 `json:"shard_id"`
}

// FlushResponse confirms the flush.
type FlushResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
