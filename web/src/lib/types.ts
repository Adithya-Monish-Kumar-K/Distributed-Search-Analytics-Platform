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
  shard_id?: number;
}

export interface SearchResponse {
  results: SearchResult[];
  total: number;
  took_ms: number;
  cache_hit: boolean;
  query: string;
}

// Raw shape returned by the Go search handler (before UI normalisation).
export interface RawSearchResult {
  doc_id?: string;
  id?: string;
  score?: number;
  title?: string;
  snippet?: string;
  shard_id?: number;
}

export interface RawSearchResponse {
  results?: RawSearchResult[];
  total?: number;
  total_hits?: number;
  took_ms?: number;
  cache_hit?: boolean;
  query?: string;
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

// Normalised analytics data used by the UI.
// The API transform layer maps the Go backend response into this shape.
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

// Raw shape returned by the Go backend GET /api/v1/analytics
export interface RawAnalyticsResponse {
  total_searches?: number;
  total_docs_indexed?: number;
  cache_hits?: number;
  cache_misses?: number;
  zero_result_count?: number;
  avg_latency_ms?: number;
  p50_latency_ms?: number;
  p95_latency_ms?: number;
  p99_latency_ms?: number;
  top_queries?: { query: string; count: number }[];
  zero_result_queries?: { query: string; count: number }[];
  queries_per_minute?: number;
}

// ─── Cache ──────────────────────────────────────────────────

// Normalised cache stats used by the UI.
export interface CacheStats {
  hits: number;
  misses: number;
  hit_rate: number;
  size: number;
  evictions: number;
}

// Raw shape returned by the Go backend GET /api/v1/cache/stats
export interface RawCacheStatsResponse {
  hits?: number;
  misses?: number;
  total?: number;
  hit_rate?: string; // e.g. "73.2%"
  size?: number;
  evictions?: number;
  status?: string;   // "disabled" when cache is off
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
  key?: string;
  api_key?: string;
  id?: string;
  name?: string;
  message?: string;
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
