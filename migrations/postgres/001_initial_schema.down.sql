DROP TRIGGER IF EXISTS documents_updated_at ON documents;
DROP TRIGGER IF EXISTS shards_updated_at ON shards;
DROP FUNCTION IF EXISTS update_updated_at();
DROP TABLE IF EXISTS ingestion_log;
DROP TABLE IF EXISTS index_versions;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS shards;