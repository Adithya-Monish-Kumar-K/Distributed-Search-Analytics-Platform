"use client";

import { useState, useCallback, useRef } from "react";
import {
  Search as SearchIcon,
  Clock,
  Zap,
  Database,
  FileText,
  X,
} from "lucide-react";
import { search } from "@/lib/api";
import type { SearchResponse, SearchResult } from "@/lib/types";
import LoadingSpinner from "@/components/loading-spinner";
import ErrorAlert from "@/components/error-alert";
import EmptyState from "@/components/empty-state";

export default function SearchPage() {
  const [query, setQuery] = useState("");
  const [limit, setLimit] = useState(10);
  const [results, setResults] = useState<SearchResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [history, setHistory] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleSearch = useCallback(
    async (q?: string) => {
      const searchQuery = q ?? query;
      if (!searchQuery.trim()) return;

      setLoading(true);
      setError(null);

      try {
        const res = await search(searchQuery.trim(), limit);
        setResults(res);
        setHistory((prev) => {
          const next = [searchQuery, ...prev.filter((h) => h !== searchQuery)];
          return next.slice(0, 10);
        });
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Search request failed",
        );
      } finally {
        setLoading(false);
      }
    },
    [query, limit],
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Search</h1>
        <p className="mt-1 text-sm text-gray-500">
          Full-text search with BM25 ranking. Supports AND, OR, NOT operators.
        </p>
      </div>

      {/* Search Bar */}
      <div className="rounded-xl border bg-white p-6 shadow-sm">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            handleSearch();
          }}
          className="space-y-4"
        >
          <div className="relative">
            <SearchIcon className="absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-gray-400" />
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder='Search documents... (e.g. "distributed AND search NOT monolithic")'
              className="input-base pl-12 pr-10"
              autoFocus
            />
            {query && (
              <button
                type="button"
                onClick={() => {
                  setQuery("");
                  inputRef.current?.focus();
                }}
                className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>

          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <label htmlFor="limit" className="text-sm text-gray-600">
                Results:
              </label>
              <select
                id="limit"
                value={limit}
                onChange={(e) => setLimit(Number(e.target.value))}
                className="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm shadow-sm focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/20"
              >
                {[5, 10, 25, 50, 100].map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </div>
            <button type="submit" className="btn-primary" disabled={loading}>
              <SearchIcon className="h-4 w-4" />
              Search
            </button>
          </div>
        </form>

        {/* Recent queries */}
        {history.length > 0 && (
          <div className="mt-4 flex flex-wrap items-center gap-2 border-t pt-4">
            <span className="text-xs text-gray-400">Recent:</span>
            {history.map((h) => (
              <button
                key={h}
                onClick={() => {
                  setQuery(h);
                  handleSearch(h);
                }}
                className="rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-600 transition-colors hover:bg-gray-200"
              >
                {h}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Status Bar */}
      {results && (
        <div className="flex flex-wrap items-center gap-4 text-sm text-gray-500">
          <span className="flex items-center gap-1">
            <FileText className="h-4 w-4" />
            {results.total} result{results.total !== 1 ? "s" : ""}
          </span>
          <span className="flex items-center gap-1">
            <Clock className="h-4 w-4" />
            {(results.took_ms ?? 0).toFixed(1)}ms
          </span>
          <span className="flex items-center gap-1">
            {results.cache_hit ? (
              <>
                <Database className="h-4 w-4 text-emerald-500" />
                <span className="text-emerald-600">Cache hit</span>
              </>
            ) : (
              <>
                <Zap className="h-4 w-4 text-amber-500" />
                <span className="text-amber-600">Live query</span>
              </>
            )}
          </span>
        </div>
      )}

      {/* Error */}
      {error && <ErrorAlert message={error} onRetry={() => handleSearch()} />}

      {/* Loading */}
      {loading && <LoadingSpinner message="Searching..." />}

      {/* Results */}
      {results && !loading && (
        <div className="space-y-3">
          {results.results.length === 0 ? (
            <EmptyState
              icon={<SearchIcon className="h-12 w-12" />}
              title="No results found"
              description={`No documents matched "${results.query}". Try different keywords or boolean operators.`}
            />
          ) : (
            results.results.map((result, index) => (
              <ResultCard key={result.id} result={result} rank={index + 1} />
            ))
          )}
        </div>
      )}

      {/* Initial state */}
      {!results && !loading && !error && (
        <EmptyState
          icon={<SearchIcon className="h-12 w-12" />}
          title="Start searching"
          description="Enter a query above to search across all indexed documents."
        />
      )}
    </div>
  );
}

function ResultCard({ result, rank }: { result: SearchResult; rank: number }) {
  return (
    <div className="group rounded-xl border bg-white p-5 shadow-sm transition-shadow hover:shadow-md">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <span className="flex h-6 w-6 items-center justify-center rounded bg-gray-100 text-xs font-medium text-gray-500">
              {rank}
            </span>
            <h3 className="text-base font-semibold text-gray-900 group-hover:text-brand-600">
              {result.title}
            </h3>
          </div>
          {result.snippet && (
            <p className="mt-2 line-clamp-2 text-sm leading-relaxed text-gray-600">
              {result.snippet}
            </p>
          )}
          <div className="mt-3 flex items-center gap-3 text-xs text-gray-400">
            <span className="font-mono">{result.id.slice(0, 8)}…</span>
            <span>Shard {result.shard_id}</span>
          </div>
        </div>
        <div className="text-right">
          <div className="rounded-lg bg-brand-50 px-3 py-1.5">
            <span className="text-sm font-semibold text-brand-700">
              {(result.score ?? 0).toFixed(3)}
            </span>
          </div>
          <p className="mt-1 text-xs text-gray-400">BM25 score</p>
        </div>
      </div>
    </div>
  );
}
