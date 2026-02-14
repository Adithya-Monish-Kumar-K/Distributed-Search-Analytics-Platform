"use client";

import { useEffect, useState, useCallback } from "react";
import {
  BarChart3,
  RefreshCcw,
  Search,
  Clock,
  TrendingUp,
  AlertTriangle,
} from "lucide-react";
import { getAnalytics } from "@/lib/api";
import type { AnalyticsData } from "@/lib/types";
import StatsCard from "@/components/stats-card";
import LoadingSpinner from "@/components/loading-spinner";
import ErrorAlert from "@/components/error-alert";
import EmptyState from "@/components/empty-state";

export default function AnalyticsPage() {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await getAnalytics();
      setData(res);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to fetch analytics",
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 10_000);
    return () => clearInterval(interval);
  }, [fetchData]);

  if (loading && !data) {
    return <LoadingSpinner message="Loading analytics..." />;
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Analytics</h1>
          <p className="mt-1 text-sm text-gray-500">
            Query performance, latency distribution, and usage patterns
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

      {!data && !loading && !error && (
        <EmptyState
          icon={<BarChart3 className="h-12 w-12" />}
          title="No analytics data"
          description="Analytics will appear here once search queries have been executed."
        />
      )}

      {data && (
        <>
          {/* Stats Cards */}
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatsCard
              title="Total Queries"
              value={data.total_queries}
              icon={Search}
              color="brand"
            />
            <StatsCard
              title="Throughput"
              value={`${data.queries_per_second.toFixed(1)}/s`}
              icon={TrendingUp}
              color="emerald"
            />
            <StatsCard
              title="Avg Latency"
              value={`${data.avg_latency_ms.toFixed(1)}ms`}
              icon={Clock}
              color="amber"
            />
            <StatsCard
              title="Error Rate"
              value={`${(data.error_rate * 100).toFixed(2)}%`}
              icon={AlertTriangle}
              color={data.error_rate > 0.05 ? "red" : "emerald"}
            />
          </div>

          {/* Latency Percentiles */}
          <section>
            <h2 className="mb-4 text-lg font-semibold text-gray-900">
              Latency Percentiles
            </h2>
            <div className="rounded-xl border bg-white p-6 shadow-sm">
              <div className="space-y-4">
                {[
                  { label: "P50 (Median)", value: data.p50_latency_ms, color: "bg-emerald-500" },
                  { label: "P95", value: data.p95_latency_ms, color: "bg-amber-500" },
                  { label: "P99", value: data.p99_latency_ms, color: "bg-red-500" },
                ].map(({ label, value, color }) => {
                  const maxVal = Math.max(data.p99_latency_ms * 1.2, 1);
                  const pct = (value / maxVal) * 100;
                  return (
                    <div key={label}>
                      <div className="mb-1 flex items-center justify-between text-sm">
                        <span className="font-medium text-gray-700">
                          {label}
                        </span>
                        <span className="font-mono text-gray-900">
                          {value.toFixed(2)}ms
                        </span>
                      </div>
                      <div className="h-3 w-full overflow-hidden rounded-full bg-gray-100">
                        <div
                          className={`h-full rounded-full ${color} transition-all duration-500`}
                          style={{ width: `${pct}%` }}
                        />
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </section>

          {/* Cache Performance */}
          <section>
            <h2 className="mb-4 text-lg font-semibold text-gray-900">
              Cache Performance
            </h2>
            <div className="rounded-xl border bg-white p-6 shadow-sm">
              <div className="flex items-center gap-8">
                {/* Ring chart */}
                <div className="relative h-32 w-32 shrink-0">
                  <svg viewBox="0 0 36 36" className="h-full w-full -rotate-90">
                    <path
                      d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                      fill="none"
                      stroke="#e5e7eb"
                      strokeWidth="3"
                    />
                    <path
                      d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                      fill="none"
                      stroke="#6366f1"
                      strokeWidth="3"
                      strokeDasharray={`${data.cache_hit_rate * 100}, 100`}
                      strokeLinecap="round"
                    />
                  </svg>
                  <div className="absolute inset-0 flex flex-col items-center justify-center">
                    <span className="text-2xl font-bold text-gray-900">
                      {(data.cache_hit_rate * 100).toFixed(0)}%
                    </span>
                    <span className="text-xs text-gray-500">Hit Rate</span>
                  </div>
                </div>
                <div className="flex-1 space-y-3 text-sm">
                  <div className="flex justify-between">
                    <span className="text-gray-500">Cache Hit Rate</span>
                    <span className="font-semibold text-gray-900">
                      {(data.cache_hit_rate * 100).toFixed(1)}%
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-500">Queries Per Second</span>
                    <span className="font-semibold text-gray-900">
                      {data.queries_per_second.toFixed(1)}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-500">Total Queries</span>
                    <span className="font-semibold text-gray-900">
                      {data.total_queries.toLocaleString()}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* Top Queries */}
          {data.top_queries && data.top_queries.length > 0 && (
            <section>
              <h2 className="mb-4 text-lg font-semibold text-gray-900">
                Top Queries
              </h2>
              <div className="overflow-hidden rounded-xl border bg-white shadow-sm">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b bg-gray-50">
                      <th className="px-6 py-3 font-medium text-gray-500">
                        #
                      </th>
                      <th className="px-6 py-3 font-medium text-gray-500">
                        Query
                      </th>
                      <th className="px-6 py-3 font-medium text-gray-500">
                        Executions
                      </th>
                      <th className="px-6 py-3 font-medium text-gray-500">
                        Avg Latency
                      </th>
                      <th className="px-6 py-3 font-medium text-gray-500">
                        Share
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    {data.top_queries.map((q, i) => (
                      <tr key={i} className="hover:bg-gray-50">
                        <td className="px-6 py-3 text-gray-400">{i + 1}</td>
                        <td className="px-6 py-3 font-mono text-gray-900">
                          {q.query}
                        </td>
                        <td className="px-6 py-3 text-gray-600">
                          {q.count.toLocaleString()}
                        </td>
                        <td className="px-6 py-3 text-gray-600">
                          {q.avg_latency_ms.toFixed(1)}ms
                        </td>
                        <td className="px-6 py-3">
                          <div className="flex items-center gap-2">
                            <div className="h-2 w-24 overflow-hidden rounded-full bg-gray-100">
                              <div
                                className="h-full rounded-full bg-brand-500"
                                style={{
                                  width: `${(q.count / data.total_queries) * 100}%`,
                                }}
                              />
                            </div>
                            <span className="text-xs text-gray-400">
                              {((q.count / data.total_queries) * 100).toFixed(
                                1,
                              )}
                              %
                            </span>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        </>
      )}
    </div>
  );
}
