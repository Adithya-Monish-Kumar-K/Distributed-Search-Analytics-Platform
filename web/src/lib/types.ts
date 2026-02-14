// ─── Domain Types ───────────────────────────────────────────

export interface Document {
  id: string;
  title: string;
  body?: string;
  content_hash: string;
  content_size: number;
  shard_id: number;
  status: "PENDING" | "INDEXING" | "INDEXED" | "FAILED" | "DELETED";
  error_message?: string;
  retry_count: number;
  created_at: string;
  updated_at: string;
  indexed_at?: string;
}

export interface SearchResult {
  id: string;
  title: string;
  score: number;
  snippet?: string;
  shard_id: number;
}

export interface SearchResponse {
  results: SearchResult[];
  total: number;
  took_ms: number;
  cache_hit: boolean;
  query: string;
}

export interface IngestRequest {
  title: string;
  body: string;
  idempotency_key?: string;
}

export interface IngestResponse {
  id: string;
  status: string;
}

// ─── Analytics ──────────────────────────────────────────────

export interface AnalyticsData {
  total_queries: number;
  queries_per_second: number;
  avg_latency_ms: number;
  p50_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
  cache_hit_rate: number;
  top_queries: TopQuery[];
  error_rate: number;
}

export interface TopQuery {
  query: string;
  count: number;
  avg_latency_ms: number;
}

// ─── Cache ──────────────────────────────────────────────────

export interface CacheStats {
  hits: number;
  misses: number;
  hit_rate: number;
  size: number;
  evictions: number;
}

// ─── Health ─────────────────────────────────────────────────

export interface HealthCheck {
  status: "up" | "down" | "degraded";
  components?: Record<string, ComponentHealth>;
}

export interface ComponentHealth {
  status: "up" | "down";
  message?: string;
  latency_ms?: number;
}

// ─── API Keys ───────────────────────────────────────────────

export interface ApiKey {
  id: string;
  name: string;
  rate_limit: number;
  is_active: boolean;
  created_at: string;
  expires_at?: string;
}

export interface CreateApiKeyRequest {
  name: string;
  rate_limit: number;
  expires_in?: string;
}

export interface CreateApiKeyResponse {
  key: string;
  id: string;
  name: string;
}

// ─── Misc ───────────────────────────────────────────────────

export type ServiceStatus = "healthy" | "unhealthy" | "unknown";

export interface PlatformOverview {
  ingestion: ServiceStatus;
  indexer: ServiceStatus;
  searcher: ServiceStatus;
  gateway: ServiceStatus;
  documentCount: number;
  cacheStats: CacheStats | null;
  analytics: AnalyticsData | null;
}
