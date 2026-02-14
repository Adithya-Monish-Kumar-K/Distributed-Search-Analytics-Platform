/**
 * Catch-all API proxy route handler.
 *
 * Replaces Next.js rewrites so that backend connection errors are handled
 * gracefully instead of flooding the terminal with ECONNREFUSED stack traces.
 *
 * URL pattern: /api/proxy/{service}/{backend-path}
 *   e.g. /api/proxy/gateway/health/ready  → http://localhost:8082/health/ready
 *        /api/proxy/search/api/v1/search  → http://localhost:8080/api/v1/search
 *        /api/proxy/ingest/api/v1/documents → http://localhost:8081/api/v1/documents
 */

import { NextRequest, NextResponse } from "next/server";

/** Backend service URL mapping. */
const SERVICE_URLS: Record<string, string> = {
  gateway: process.env.NEXT_PUBLIC_GATEWAY_URL || "http://localhost:8082",
  search: process.env.NEXT_PUBLIC_SEARCH_URL || "http://localhost:8080",
  ingest: process.env.NEXT_PUBLIC_INGESTION_URL || "http://localhost:8081",
};

/**
 * Resolves the target URL for a given proxy path.
 * The first segment is the service name, the rest is the backend path.
 */
function resolveTarget(pathSegments: string[]): { url: string; service: string } | null {
  if (pathSegments.length < 1) return null;
  const [service, ...rest] = pathSegments;
  const baseUrl = SERVICE_URLS[service];
  if (!baseUrl) return null;
  return { url: `${baseUrl}/${rest.join("/")}`, service };
}

/** Forward a request to the appropriate backend, catching connection errors. */
async function proxyRequest(req: NextRequest, { params }: { params: { path: string[] } }) {
  const target = resolveTarget(params.path);
  if (!target) {
    return NextResponse.json(
      { error: "Unknown service", available: Object.keys(SERVICE_URLS) },
      { status: 400 },
    );
  }

  // Build the target URL preserving query string
  const url = new URL(target.url);
  const searchParams = req.nextUrl.searchParams;
  searchParams.forEach((value, key) => url.searchParams.set(key, value));

  // Forward headers, dropping host and internal Next.js headers
  const headers = new Headers();
  req.headers.forEach((value, key) => {
    if (!["host", "connection", "transfer-encoding"].includes(key.toLowerCase())) {
      headers.set(key, value);
    }
  });

  try {
    const body = req.method !== "GET" && req.method !== "HEAD"
      ? await req.text()
      : undefined;

    const response = await fetch(url.toString(), {
      method: req.method,
      headers,
      body,
      // Prevent Next.js from caching proxy responses
      cache: "no-store",
    });

    // Stream the response back
    const responseHeaders = new Headers();
    response.headers.forEach((value, key) => {
      if (!["transfer-encoding", "content-encoding"].includes(key.toLowerCase())) {
        responseHeaders.set(key, value);
      }
    });

    const responseBody = await response.text();
    return new NextResponse(responseBody, {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
    });
  } catch (err) {
    // Connection refused or network error — return a clean 503
    const message = err instanceof Error ? err.message : "Unknown error";
    const isConnRefused = message.includes("ECONNREFUSED") ||
      message.includes("fetch failed") ||
      message.includes("connect");

    return NextResponse.json(
      {
        error: isConnRefused
          ? `Service "${target.service}" is not reachable`
          : `Proxy error: ${message}`,
        service: target.service,
        status: "unavailable",
      },
      { status: 503 },
    );
  }
}

// Handle all HTTP methods
export const GET = proxyRequest;
export const POST = proxyRequest;
export const PUT = proxyRequest;
export const DELETE = proxyRequest;
export const PATCH = proxyRequest;
