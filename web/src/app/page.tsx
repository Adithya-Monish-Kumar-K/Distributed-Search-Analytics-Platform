"use client";

import { useEffect, useState, useCallback } from "react";
import {
  FileText,
  Search,
  Zap,
  Database,
  Activity,
  RefreshCcw,
  Server,
} from "lucide-react";
import StatsCard from "@/components/stats-card";
import HealthBadge from "@/components/health-badge";
import LoadingSpinner from "@/components/loading-spinner";
import ErrorAlert from "@/components/error-alert";
import { getAnalytics, getCacheStats, getHealth } from "@/lib/api";
import type {
  AnalyticsData,
  CacheStats,
  ServiceStatus,
} from "@/lib/types";

interface ServiceInfo {
  name: string;
  status: ServiceStatus;
  port: string;
}

export default function DashboardPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [analytics, setAnalytics] = useState<AnalyticsData | null>(null);
  const [cache, setCache] = useState<CacheStats | null>(null);
  const [services, setServices] = useState<ServiceInfo[]>([]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);

    // Fetch all data in parallel, don't fail on individual errors
    const results = await Promise.allSettled([
      getAnalytics(),
      getCacheStats(),
      getHealth("search"),
      getHealth("ingestion"),
      getHealth("gateway"),
    ]);

    const [analyticsRes, cacheRes, searchHealth, ingestionHealth, gatewayHealth] =
      results;

    if (analyticsRes.status === "fulfilled") setAnalytics(analyticsRes.value);
    if (cacheRes.status === "fulfilled") setCache(cacheRes.value);

    const svcList: ServiceInfo[] = [
      {
        name: "Search Service",
        status: searchHealth.status === "fulfilled" ? "healthy" : "unhealthy",
        port: "8080",
      },
      {
        name: "Ingestion Service",
        status:
          ingestionHealth.status === "fulfilled" ? "healthy" : "unhealthy",
        port: "8081",
      },
      {
        name: "API Gateway",
        status:
          gatewayHealth.status === "fulfilled" ? "healthy" : "unhealthy",
        port: "8082",
      },
    ];
    setServices(svcList);

    // Only show error if *everything* failed
    const allFailed = results.every((r) => r.status === "rejected");
    if (allFailed) {
      setError(
        "Could not connect to any service. Make sure the backend is running.",
      );
    }

    setLoading(false);
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 15_000);
    return () => clearInterval(interval);
  }, [fetchData]);

  if (loading && !analytics && !cache) {
    return <LoadingSpinner message="Connecting to services..." />;
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <p className="mt-1 text-sm text-gray-500">
            Platform overview and service health
          </p>
        </div>
        <button
          onClick={fetchData}
          className="btn-secondary"
          disabled={loading}
        >
          <RefreshCcw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          Refresh
        </button>
      </div>

      {error && <ErrorAlert message={error} onRetry={fetchData} />}

      {/* Service Health */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-gray-900">
          Service Health
        </h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {services.map((svc) => (
            <div
              key={svc.name}
              className="flex items-center justify-between rounded-xl border bg-white p-5 shadow-sm"
            >
              <div className="flex items-center gap-3">
                <div className="rounded-lg bg-gray-100 p-2">
                  <Server className="h-5 w-5 text-gray-600" />
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">
                    {svc.name}
                  </p>
                  <p className="text-xs text-gray-500">Port {svc.port}</p>
                </div>
              </div>
              <HealthBadge
                status={svc.status}
                label={svc.status === "healthy" ? "Healthy" : "Down"}
              />
            </div>
          ))}
        </div>
      </section>

      {/* Stats Grid */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-gray-900">
          Key Metrics
        </h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatsCard
            title="Total Queries"
            value={analytics?.total_queries ?? 0}
            icon={Search}
            color="brand"
            subtitle={
              analytics?.queries_per_second != null
                ? `${analytics.queries_per_second.toFixed(1)} queries/sec`
                : undefined
            }
          />
          <StatsCard
            title="Avg Latency"
            value={
              analytics?.avg_latency_ms != null
                ? `${analytics.avg_latency_ms.toFixed(1)}ms`
                : "—"
            }
            icon={Zap}
            color="amber"
            subtitle={
              analytics?.p99_latency_ms != null
                ? `P99: ${analytics.p99_latency_ms.toFixed(1)}ms`
                : undefined
            }
          />
          <StatsCard
            title="Cache Hit Rate"
            value={
              cache?.hit_rate != null && !isNaN(cache.hit_rate)
                ? `${(cache.hit_rate * 100).toFixed(1)}%`
                : analytics?.cache_hit_rate != null && !isNaN(analytics.cache_hit_rate)
                  ? `${(analytics.cache_hit_rate * 100).toFixed(1)}%`
                  : "0%"
            }
            icon={Database}
            color="emerald"
            subtitle={
              cache
                ? `${cache.hits ?? 0} hits / ${cache.misses ?? 0} misses`
                : undefined
            }
          />
          <StatsCard
            title="Error Rate"
            value={
              analytics?.error_rate != null && !isNaN(analytics.error_rate)
                ? `${(analytics.error_rate * 100).toFixed(2)}%`
                : "0%"
            }
            icon={Activity}
            color={
              analytics?.error_rate != null && analytics.error_rate > 0.05
                ? "red"
                : "emerald"
            }
          />
        </div>
      </section>

      {/* Latency Breakdown */}
      {analytics && (
        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900">
            Latency Distribution
          </h2>
          <div className="rounded-xl border bg-white p-6 shadow-sm">
            <div className="grid grid-cols-3 gap-8">
              {[
                { label: "P50", value: analytics.p50_latency_ms },
                { label: "P95", value: analytics.p95_latency_ms },
                { label: "P99", value: analytics.p99_latency_ms },
              ].map(({ label, value }) => (
                <div key={label} className="text-center">
                  <p className="text-sm font-medium text-gray-500">{label}</p>
                  <p className="mt-1 text-2xl font-bold text-gray-900">
                    {(value ?? 0).toFixed(1)}
                    <span className="ml-1 text-sm font-normal text-gray-400">
                      ms
                    </span>
                  </p>
                  <div className="mx-auto mt-3 h-2 w-full max-w-32 overflow-hidden rounded-full bg-gray-100">
                    <div
                      className="h-full rounded-full bg-brand-500 transition-all"
                      style={{
                        width: `${Math.min(((value ?? 0) / (analytics.p99_latency_ms || 1)) * 100, 100)}%`,
                      }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>
      )}

      {/* Top Queries */}
      {analytics && analytics.top_queries && analytics.top_queries.length > 0 && (
        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900">
            Top Queries
          </h2>
          <div className="overflow-hidden rounded-xl border bg-white shadow-sm">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b bg-gray-50">
                  <th className="px-6 py-3 font-medium text-gray-500">
                    Query
                  </th>
                  <th className="px-6 py-3 font-medium text-gray-500">
                    Count
                  </th>
                  <th className="px-6 py-3 font-medium text-gray-500">
                    Avg Latency
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {analytics.top_queries.map((q, i) => (
                  <tr key={i} className="hover:bg-gray-50">
                    <td className="px-6 py-3 font-mono text-gray-900">
                      {q.query}
                    </td>
                    <td className="px-6 py-3 text-gray-600">{q.count}</td>
                    <td className="px-6 py-3 text-gray-600">
                      {(q.avg_latency_ms ?? 0).toFixed(1)}ms
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  );
}
