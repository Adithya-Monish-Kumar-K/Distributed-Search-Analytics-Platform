// Package ingestion defines the request/response types and Kafka event schemas
// used by the document ingestion pipeline.
package ingestion

import "time"

// IngestRequest is the JSON body accepted by the ingestion HTTP endpoint.
type IngestRequest struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	IdempotencyKey string `json:"idempotency_key"`
}

// IngestResponse is returned to the caller after a document is accepted.
type IngestResponse struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
	ShardID    int    `json:"shard_id"`
}

// IngestEvent is the Kafka message payload produced after a document is
// persisted and ready for indexing.
type IngestEvent struct {
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	ShardID    int       `json:"shard_id"`
	IngestedAt time.Time `json:"ingested_at"`
}
