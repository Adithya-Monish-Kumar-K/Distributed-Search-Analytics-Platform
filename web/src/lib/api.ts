import type {
  SearchResponse,
  IngestRequest,
  IngestResponse,
  AnalyticsData,
  CacheStats,
  HealthCheck,
  ApiKey,
  CreateApiKeyRequest,
  CreateApiKeyResponse,
  Document,
  RawAnalyticsResponse,
  RawCacheStatsResponse,
  RawSearchResponse,
} from "./types";

// ─── Base fetcher ───────────────────────────────────────────

// All requests route through the Next.js API proxy at /api/proxy/{service}/...
// This avoids CORS issues and handles backend connection errors gracefully
// instead of flooding the dev-server console with ECONNREFUSED stack traces.
const SEARCH_BASE = "/api/proxy/search";
const INGEST_BASE = "/api/proxy/ingest";
const GATEWAY_BASE = "/api/proxy/gateway";

async function fetchJSON<T>(
  url: string,
  opts?: RequestInit & { apiKey?: string },
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(opts?.headers as Record<string, string>),
  };
  if (opts?.apiKey) {
    headers["Authorization"] = `Bearer ${opts.apiKey}`;
  }
  const res = await fetch(url, { ...opts, headers });
  if (!res.ok) {
    // Try to parse a JSON error body and extract a human-readable message
    const text = await res.text().catch(() => res.statusText);
    let message = `${res.status}: ${text}`;
    try {
      const body = JSON.parse(text);
      if (body.error) {
        message = body.error;
      } else if (body.status === "unavailable") {
        message = `Service ${body.service ?? ""} is not reachable`;
      }
    } catch {
      // text wasn't JSON, use as-is
    }
    throw new Error(message);
  }
  // Some endpoints return 202 with no meaningful JSON body
  if (res.status === 204 || res.headers.get("content-length") === "0") {
    return {} as T;
  }
  return res.json();
}

// ─── Search ─────────────────────────────────────────────────

/** Transform the raw Go search response into the normalised SearchResponse shape */
function transformSearchResponse(raw: RawSearchResponse): SearchResponse {
  return {
    query: raw.query ?? "",
    total: raw.total ?? raw.total_hits ?? 0,
    took_ms: raw.took_ms ?? 0,
    cache_hit: raw.cache_hit ?? false,
    results: (raw.results ?? []).map((r) => ({
      id: r.doc_id ?? r.id ?? "",
      title: r.title ?? "",
      score: r.score ?? 0,
      snippet: r.snippet,
      shard_id: r.shard_id,
    })),
  };
}

export async function search(
  query: string,
  limit = 10,
): Promise<SearchResponse> {
  const params = new URLSearchParams({ q: query, limit: String(limit) });
  const raw = await fetchJSON<RawSearchResponse>(
    `${SEARCH_BASE}/api/v1/search?${params}`,
  );
  return transformSearchResponse(raw);
}

// ─── Ingestion ──────────────────────────────────────────────

export async function ingestDocument(
  doc: IngestRequest,
): Promise<IngestResponse> {
  return fetchJSON<IngestResponse>(`${INGEST_BASE}/api/v1/documents`, {
    method: "POST",
    body: JSON.stringify(doc),
  });
}

// ─── Documents (via gateway) ────────────────────────────────

export async function getDocuments(
  apiKey?: string,
): Promise<{ documents: Document[] }> {
  return fetchJSON<{ documents: Document[] }>(
    `${GATEWAY_BASE}/api/v1/documents`,
    { apiKey },
  );
}

export async function getDocument(
  id: string,
  apiKey?: string,
): Promise<Document> {
  return fetchJSON<Document>(`${GATEWAY_BASE}/api/v1/documents/${id}`, {
    apiKey,
  });
}

// ─── Analytics ──────────────────────────────────────────────

/** Transform the raw Go backend response into the normalised AnalyticsData shape */
function transformAnalytics(raw: RawAnalyticsResponse): AnalyticsData {
  const cacheHits = raw.cache_hits ?? 0;
  const cacheMisses = raw.cache_misses ?? 0;
  const cacheTotal = cacheHits + cacheMisses;
  const cacheHitRate = cacheTotal > 0 ? cacheHits / cacheTotal : 0;

  return {
    total_queries: raw.total_searches ?? 0,
    queries_per_second: (raw.queries_per_minute ?? 0) / 60,
    avg_latency_ms: raw.avg_latency_ms ?? 0,
    p50_latency_ms: raw.p50_latency_ms ?? 0,
    p95_latency_ms: raw.p95_latency_ms ?? 0,
    p99_latency_ms: raw.p99_latency_ms ?? 0,
    cache_hit_rate: cacheHitRate,
    error_rate: 0, // backend doesn't track error rate yet
    top_queries: (raw.top_queries ?? []).map((q) => ({
      query: q.query ?? "",
      count: q.count ?? 0,
      avg_latency_ms: 0, // not tracked per-query by backend
    })),
  };
}

export async function getAnalytics(): Promise<AnalyticsData> {
  const raw = await fetchJSON<RawAnalyticsResponse>(`${SEARCH_BASE}/api/v1/analytics`);
  return transformAnalytics(raw);
}

// ─── Cache ──────────────────────────────────────────────────

/** Transform the raw Go backend response into the normalised CacheStats shape */
function transformCacheStats(raw: RawCacheStatsResponse): CacheStats {
  // The Go backend returns hit_rate as a formatted string like "73.2%"
  let hitRate = 0;
  if (typeof raw.hit_rate === "string") {
    hitRate = parseFloat(raw.hit_rate.replace("%", "")) / 100;
    if (isNaN(hitRate)) hitRate = 0;
  } else if (typeof raw.hit_rate === "number") {
    hitRate = raw.hit_rate;
  }

  return {
    hits: raw.hits ?? 0,
    misses: raw.misses ?? 0,
    hit_rate: hitRate,
    size: raw.size ?? (raw.total ?? 0),
    evictions: raw.evictions ?? 0,
  };
}

export async function getCacheStats(): Promise<CacheStats> {
  const raw = await fetchJSON<RawCacheStatsResponse>(`${SEARCH_BASE}/api/v1/cache/stats`);
  return transformCacheStats(raw);
}

export async function invalidateCache(): Promise<void> {
  await fetchJSON<void>(`${SEARCH_BASE}/api/v1/cache/invalidate`, {
    method: "POST",
  });
}

// ─── Health ─────────────────────────────────────────────────

export async function getHealth(
  service: "search" | "ingestion" | "gateway",
): Promise<HealthCheck> {
  const bases: Record<string, string> = {
    search: SEARCH_BASE,
    ingestion: INGEST_BASE,
    gateway: GATEWAY_BASE,
  };
  const base = bases[service];

  // Ingestion and Gateway only expose GET /health (no /health/ready or /health/live).
  // Search has /health/ready and /health/live via the health.Checker.
  if (service === "ingestion" || service === "gateway") {
    return fetchJSON<HealthCheck>(`${base}/health`);
  }

  try {
    return await fetchJSON<HealthCheck>(`${base}/health/ready`);
  } catch {
    return fetchJSON<HealthCheck>(`${base}/health/live`);
  }
}

// ─── API Keys (via gateway) ─────────────────────────────────

export async function createApiKey(
  req: CreateApiKeyRequest,
  apiKey?: string,
): Promise<CreateApiKeyResponse> {
  return fetchJSON<CreateApiKeyResponse>(`${GATEWAY_BASE}/api/v1/admin/keys`, {
    method: "POST",
    body: JSON.stringify(req),
    apiKey,
  });
}

export async function listApiKeys(
  apiKey?: string,
): Promise<{ keys: ApiKey[] }> {
  const raw = await fetchJSON<{ keys: RawApiKey[]; count?: number }>(
    `${GATEWAY_BASE}/api/v1/admin/keys`,
    { apiKey },
  );
  return { keys: (raw.keys ?? []).map(transformApiKey) };
}

/** Raw shape returned by the Go gateway — handles both old (uppercase) and new (lowercase JSON-tagged) formats */
interface RawApiKey {
  // New format (with json tags)
  id?: string;
  name?: string;
  rate_limit?: number;
  is_active?: boolean;
  created_at?: string;
  expires_at?: string | null;
  // Old format (Go default, uppercase)
  ID?: string;
  Name?: string;
  RateLimit?: number;
  IsActive?: boolean;
  CreatedAt?: string;
  ExpiresAt?: string | null;
}

/** Normalise a raw API key object into the UI-expected shape */
function transformApiKey(raw: RawApiKey): ApiKey {
  return {
    id: raw.id ?? raw.ID ?? "",
    name: raw.name ?? raw.Name ?? "",
    rate_limit: raw.rate_limit ?? raw.RateLimit ?? 0,
    is_active: raw.is_active ?? raw.IsActive ?? true, // ListKeys only returns active keys
    created_at: raw.created_at ?? raw.CreatedAt ?? "",
    expires_at: raw.expires_at ?? raw.ExpiresAt ?? undefined,
  };
}
