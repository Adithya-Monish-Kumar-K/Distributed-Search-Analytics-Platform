CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE,
    title VARCHAR(1024) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    content_size INTEGER NOT NULL,
    shard_id INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'INDEXING', 'INDEXED', 'FAILED', 'DELETED')),
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    indexed_at TIMESTAMPTZ
);

CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_shard_id ON documents(shard_id);
CREATE INDEX idx_documents_content_hash ON documents(content_hash);
CREATE INDEX idx_documents_created_at ON documents(created_at);

CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER documents_updated_at
BEFORE UPDATE ON documents
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE shards (
    id INTEGER PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DRAINING', 'INACTIVE','REBALANCING')),
    node_id VARCHAR(255) NOT NULL,
    replica_node_ids TEXT[] DEFAULT '{}',
    document_count BIGINT NOT NULL DEFAULT 0,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shards_status ON shards(status);
CREATE INDEX idx_shards_node_id ON shards(node_id);

CREATE TRIGGER shards_updated_at BEFORE UPDATE ON shards FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TABLE index_versions(
    id BIGSERIAL PRIMARY KEY,
    shard_id INTEGER NOT NULL REFERENCES shards(id),
    version INTEGER NOT NULL,
    segment_count INTEGER NOT NULL DEFAULT 0,
    total_docs BIGINT NOT NULL DEFAULT 0,
    total_size_bytes BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'BUILDING' CHECK (status IN ('BUILDING', 'ACTIVE', 'MERGING', 'RETIRED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    UNIQUE(shard_id, version)
);

CREATE INDEX idx_index_versions_shard_id ON index_versions(shard_id) WHERE status = 'ACTIVE';

CREATE TABLE ingestion_log(
    id BIGSERIAL PRIMARY KEY,
    document_id UUID NOT NULL REFERENCES documents(id),
    kafka_topic VARCHAR(255) NOT NULL,
    kafka_partition INTEGER NOT NULL,
    kafka_offset BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'RECEIVED' CHECK (status IN ('RECEIVED', 'PROCESSING', 'COMPLETED', 'FAILED')),
    processor_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(kafka_topic, kafka_partition, kafka_offset)
);

CREATE INDEX idx_ingestion_log_document_id ON ingestion_log(document_id);
CREATE INDEX idx_ingestion_log_status ON ingestion_log(status);

CREATE TABLE api_keys(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    rate_limit INTEGER NOT NULL DEFAULT 100,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash) WHERE is_active = true;