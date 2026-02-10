package ingestion

import "time"

type IngestRequest struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	IdempotencyKey string `json:"idempotency_key"`
}

type IngestResponse struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
	ShardID    int    `json:"shard_id"`
}

type IngestEvent struct {
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	ShardID    int       `json:"shard_id"`
	IngestedAt time.Time `json:"ingested_at"`
}
