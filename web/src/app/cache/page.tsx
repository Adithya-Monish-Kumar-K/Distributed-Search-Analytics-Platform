"use client";

import { useEffect, useState, useCallback } from "react";
import {
  Database,
  RefreshCcw,
  Trash2,
  CheckCircle2,
  HardDrive,
  ArrowRightLeft,
} from "lucide-react";
import { getCacheStats, invalidateCache } from "@/lib/api";
import type { CacheStats } from "@/lib/types";
import StatsCard from "@/components/stats-card";
import LoadingSpinner from "@/components/loading-spinner";
import ErrorAlert from "@/components/error-alert";
import EmptyState from "@/components/empty-state";
import { formatNumber } from "@/lib/utils";

export default function CachePage() {
  const [stats, setStats] = useState<CacheStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [invalidating, setInvalidating] = useState(false);
  const [invalidated, setInvalidated] = useState(false);

  const fetchStats = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await getCacheStats();
      setStats(res);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to fetch cache stats",
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 10_000);
    return () => clearInterval(interval);
  }, [fetchStats]);

  const handleInvalidate = async () => {
    setInvalidating(true);
    setInvalidated(false);
    try {
      await invalidateCache();
      setInvalidated(true);
      setTimeout(() => setInvalidated(false), 3000);
      fetchStats();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to invalidate cache",
      );
    } finally {
      setInvalidating(false);
    }
  };

  if (loading && !stats) {
    return <LoadingSpinner message="Loading cache stats..." />;
  }

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Cache</h1>
          <p className="mt-1 text-sm text-gray-500">
            Redis query cache statistics and management
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={fetchStats}
            className="btn-secondary"
            disabled={loading}
          >
            <RefreshCcw
              className={`h-4 w-4 ${loading ? "animate-spin" : ""}`}
            />
            Refresh
          </button>
          <button
            onClick={handleInvalidate}
            className="btn-danger"
            disabled={invalidating}
          >
            {invalidating ? (
              <RefreshCcw className="h-4 w-4 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
            Flush Cache
          </button>
        </div>
      </div>

      {invalidated && (
        <div className="flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
          <CheckCircle2 className="h-4 w-4" />
          Cache invalidated successfully.
        </div>
      )}

      {error && <ErrorAlert message={error} onRetry={fetchStats} />}

      {!stats && !loading && !error && (
        <EmptyState
          icon={<Database className="h-12 w-12" />}
          title="No cache data"
          description="Cache stats will appear once the search service is running and handling queries."
        />
      )}

      {stats && (
        <>
          {/* Stats Cards */}
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatsCard
              title="Hit Rate"
              value={`${((stats.hit_rate ?? 0) * 100).toFixed(1)}%`}
              icon={Database}
              color={(stats.hit_rate ?? 0) > 0.5 ? "emerald" : "amber"}
            />
            <StatsCard
              title="Cache Hits"
              value={stats.hits}
              icon={CheckCircle2}
              color="emerald"
            />
            <StatsCard
              title="Cache Misses"
              value={stats.misses}
              icon={ArrowRightLeft}
              color="amber"
            />
            <StatsCard
              title="Evictions"
              value={stats.evictions}
              icon={Trash2}
              color="red"
            />
          </div>

          {/* Visual Breakdown */}
          <section>
            <h2 className="mb-4 text-lg font-semibold text-gray-900">
              Hit / Miss Ratio
            </h2>
            <div className="rounded-xl border bg-white p-6 shadow-sm">
              <div className="mb-4 flex items-center justify-between text-sm text-gray-600">
                <span>
                  Hits:{" "}
                  <span className="font-semibold text-emerald-600">
                    {formatNumber(stats.hits)}
                  </span>
                </span>
                <span>
                  Misses:{" "}
                  <span className="font-semibold text-amber-600">
                    {formatNumber(stats.misses)}
                  </span>
                </span>
              </div>
              <div className="flex h-8 overflow-hidden rounded-full bg-gray-100">
                <div
                  className="flex items-center justify-center bg-emerald-500 text-xs font-medium text-white transition-all duration-500"
                  style={{
                    width: `${stats.hit_rate * 100}%`,
                  }}
                >
                  {stats.hit_rate > 0.1 &&
                    `${((stats.hit_rate ?? 0) * 100).toFixed(0)}%`}
                </div>
                <div
                  className="flex items-center justify-center bg-amber-500 text-xs font-medium text-white transition-all duration-500"
                  style={{
                    width: `${(1 - stats.hit_rate) * 100}%`,
                  }}
                >
                  {1 - stats.hit_rate > 0.1 &&
                    `${((1 - (stats.hit_rate ?? 0)) * 100).toFixed(0)}%`}
                </div>
              </div>
            </div>
          </section>

          {/* Cache Details */}
          <section>
            <h2 className="mb-4 text-lg font-semibold text-gray-900">
              Details
            </h2>
            <div className="rounded-xl border bg-white shadow-sm">
              <dl className="divide-y">
                {[
                  {
                    label: "Cache Size",
                    value: `${formatNumber(stats.size)} entries`,
                    icon: HardDrive,
                  },
                  {
                    label: "Total Hits",
                    value: formatNumber(stats.hits),
                    icon: CheckCircle2,
                  },
                  {
                    label: "Total Misses",
                    value: formatNumber(stats.misses),
                    icon: ArrowRightLeft,
                  },
                  {
                    label: "Evictions",
                    value: formatNumber(stats.evictions),
                    icon: Trash2,
                  },
                  {
                    label: "Total Requests",
                    value: formatNumber(stats.hits + stats.misses),
                    icon: Database,
                  },
                ].map(({ label, value, icon: Icon }) => (
                  <div
                    key={label}
                    className="flex items-center justify-between px-6 py-4"
                  >
                    <dt className="flex items-center gap-2 text-sm text-gray-500">
                      <Icon className="h-4 w-4" />
                      {label}
                    </dt>
                    <dd className="text-sm font-semibold text-gray-900">
                      {value}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          </section>

          {/* Configuration Note */}
          <div className="rounded-xl border bg-gray-50 p-5">
            <h3 className="text-sm font-semibold text-gray-900">
              Cache Configuration
            </h3>
            <p className="mt-1 text-sm text-gray-600">
              The cache uses Redis with singleflight stampede prevention.
              Configure TTL and pool size in the YAML config:
            </p>
            <pre className="mt-3 overflow-x-auto rounded-lg bg-gray-900 p-4 text-sm text-gray-100">
              <code>{`redis:
  addr: localhost:6379
  poolSize: 10
  cacheTTL: 60s`}</code>
            </pre>
          </div>
        </>
      )}
    </div>
  );
}
