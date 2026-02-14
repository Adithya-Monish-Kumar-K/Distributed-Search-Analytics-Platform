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
    const text = await res.text().catch(() => res.statusText);
    throw new Error(`${res.status}: ${text}`);
  }
  // Some endpoints return 202 with no meaningful JSON body
  if (res.status === 204 || res.headers.get("content-length") === "0") {
    return {} as T;
  }
  return res.json();
}

// ─── Search ─────────────────────────────────────────────────

export async function search(
  query: string,
  limit = 10,
): Promise<SearchResponse> {
  const params = new URLSearchParams({ q: query, limit: String(limit) });
  return fetchJSON<SearchResponse>(
    `${SEARCH_BASE}/api/v1/search?${params}`,
  );
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

export async function getAnalytics(): Promise<AnalyticsData> {
  return fetchJSON<AnalyticsData>(`${SEARCH_BASE}/api/v1/analytics`);
}

// ─── Cache ──────────────────────────────────────────────────

export async function getCacheStats(): Promise<CacheStats> {
  return fetchJSON<CacheStats>(`${SEARCH_BASE}/api/v1/cache/stats`);
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
  try {
    return await fetchJSON<HealthCheck>(`${base}/health/ready`);
  } catch {
    try {
      return await fetchJSON<HealthCheck>(`${base}/health/live`);
    } catch {
      // ingestion uses /health
      return fetchJSON<HealthCheck>(`${base}/health`);
    }
  }
}

// ─── API Keys (via gateway) ─────────────────────────────────

export async function createApiKey(
  req: CreateApiKeyRequest,
  apiKey?: string,
): Promise<CreateApiKeyResponse> {
  return fetchJSON<CreateApiKeyResponse>(`${GATEWAY_BASE}/admin/api-keys`, {
    method: "POST",
    body: JSON.stringify(req),
    apiKey,
  });
}

export async function listApiKeys(
  apiKey?: string,
): Promise<{ keys: ApiKey[] }> {
  return fetchJSON<{ keys: ApiKey[] }>(`${GATEWAY_BASE}/admin/api-keys`, {
    apiKey,
  });
}
