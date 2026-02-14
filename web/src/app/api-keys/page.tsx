"use client";

import { useEffect, useState, useCallback } from "react";
import {
  Key,
  Plus,
  RefreshCcw,
  Trash2,
  Copy,
  CheckCircle2,
  Shield,
  Loader2,
} from "lucide-react";
import { listApiKeys, createApiKey } from "@/lib/api";
import type { ApiKey } from "@/lib/types";
import LoadingSpinner from "@/components/loading-spinner";
import ErrorAlert from "@/components/error-alert";
import EmptyState from "@/components/empty-state";
import { timeAgo } from "@/lib/utils";

export default function ApiKeysPage() {
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [storedApiKey, setStoredApiKey] = useState<string>("");
  const [initialized, setInitialized] = useState(false);

  // Load the stored key from localStorage after mount (avoids SSR mismatch).
  useEffect(() => {
    const k = localStorage.getItem("sp_api_key") ?? "";
    setStoredApiKey(k);
    setInitialized(true);
  }, []);

  const fetchKeys = useCallback(async () => {
    if (!storedApiKey) {
      setLoading(false);
      setError("Enter a gateway API key above, then click Save & Refresh.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await listApiKeys(storedApiKey);
      setKeys(res.keys ?? []);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to fetch API keys. Make sure the gateway is running.",
      );
    } finally {
      setLoading(false);
    }
  }, [storedApiKey]);

  // Only auto-fetch once localStorage has been read.
  useEffect(() => {
    if (initialized) {
      fetchKeys();
    }
  }, [fetchKeys, initialized]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">API Keys</h1>
          <p className="mt-1 text-sm text-gray-500">
            Manage API keys for authenticated gateway access
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={fetchKeys}
            className="btn-secondary"
            disabled={loading}
          >
            <RefreshCcw
              className={`h-4 w-4 ${loading ? "animate-spin" : ""}`}
            />
            Refresh
          </button>
          <button
            onClick={() => setShowCreate(true)}
            className="btn-primary"
          >
            <Plus className="h-4 w-4" />
            Create Key
          </button>
        </div>
      </div>

      {/* Stored Key Config */}
      <div className="rounded-xl border bg-white p-5 shadow-sm">
        <div className="flex items-start gap-3">
          <Shield className="mt-0.5 h-5 w-5 text-brand-600" />
          <div className="flex-1">
            <h3 className="text-sm font-semibold text-gray-900">
              Gateway Authentication
            </h3>
            <p className="mt-0.5 text-xs text-gray-500">
              Enter an existing API key to authenticate with the gateway. This
              is stored in your browser only.
            </p>
            <div className="mt-3 flex gap-2">
              <input
                type="password"
                value={storedApiKey}
                onChange={(e) => setStoredApiKey(e.target.value)}
                placeholder="Paste your API key here..."
                className="input-base max-w-md"
              />
              <button
                className="btn-secondary"
                onClick={() => {
                  localStorage.setItem("sp_api_key", storedApiKey);
                  fetchKeys();
                }}
              >
                Save &amp; Refresh
              </button>
            </div>
          </div>
        </div>
      </div>

      {error && <ErrorAlert message={error} onRetry={fetchKeys} />}

      {/* Create Form */}
      {showCreate && (
        <CreateKeyForm
          apiKey={storedApiKey}
          onClose={() => setShowCreate(false)}
          onSuccess={(newKey) => {
            setShowCreate(false);
            setStoredApiKey(newKey);
            localStorage.setItem("sp_api_key", newKey);
            fetchKeys();
          }}
        />
      )}

      {/* Keys Table */}
      {loading && keys.length === 0 ? (
        <LoadingSpinner message="Loading API keys..." />
      ) : keys.length === 0 && !error ? (
        <EmptyState
          icon={<Key className="h-12 w-12" />}
          title="No API keys"
          description="Create an API key to authenticate requests through the gateway."
          action={
            <button
              onClick={() => setShowCreate(true)}
              className="btn-primary"
            >
              <Plus className="h-4 w-4" />
              Create Key
            </button>
          }
        />
      ) : (
        keys.length > 0 && (
          <div className="overflow-hidden rounded-xl border bg-white shadow-sm">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b bg-gray-50">
                  <th className="px-6 py-3 font-medium text-gray-500">Name</th>
                  <th className="px-6 py-3 font-medium text-gray-500">
                    Status
                  </th>
                  <th className="px-6 py-3 font-medium text-gray-500">
                    Rate Limit
                  </th>
                  <th className="px-6 py-3 font-medium text-gray-500">
                    Created
                  </th>
                  <th className="px-6 py-3 font-medium text-gray-500">
                    Expires
                  </th>
                  <th className="px-6 py-3 font-medium text-gray-500"></th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {keys.map((key, idx) => (
                  <tr key={key.id ?? idx} className="hover:bg-gray-50">
                    <td className="px-6 py-4">
                      <div>
                        <p className="font-medium text-gray-900">{key.name}</p>
                        {key.id && (
                          <p className="mt-0.5 font-mono text-xs text-gray-400">
                            {key.id.slice(0, 8)}…
                          </p>
                        )}
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span
                        className={
                          key.is_active ? "badge-green" : "badge-red"
                        }
                      >
                        {key.is_active ? "Active" : "Revoked"}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-gray-600">
                      {key.rate_limit} req/min
                    </td>
                    <td className="px-6 py-4 text-gray-500">
                      {timeAgo(key.created_at)}
                    </td>
                    <td className="px-6 py-4 text-gray-500">
                      {key.expires_at
                        ? new Date(key.expires_at).toLocaleDateString()
                        : "Never"}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <button
                        className="text-gray-400 hover:text-red-600"
                        title="Revoke key"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {/* CLI Tip */}
      <div className="rounded-xl border bg-gray-50 p-5">
        <h3 className="text-sm font-semibold text-gray-900">
          CLI Alternative
        </h3>
        <p className="mt-1 text-sm text-gray-600">
          You can also manage API keys from the command line:
        </p>
        <pre className="mt-3 overflow-x-auto rounded-lg bg-gray-900 p-4 text-sm text-gray-100">
          <code>{`# Create a key
go run ./cmd/auth create --name "my-app" --rate-limit 100

# List all keys
go run ./cmd/auth list

# Revoke a key
go run ./cmd/auth -- revoke --key "<raw-key>"`}</code>
        </pre>
      </div>
    </div>
  );
}

// ─── Create Key Form ────────────────────────────────────────

function CreateKeyForm({
  apiKey,
  onClose,
  onSuccess,
}: {
  apiKey: string;
  onClose: () => void;
  onSuccess: (rawKey: string) => void;
}) {
  const [name, setName] = useState("");
  const [rateLimit, setRateLimit] = useState(100);
  const [expiresIn, setExpiresIn] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      const res = await createApiKey(
        { name, rate_limit: rateLimit, expires_in: expiresIn || undefined },
        apiKey || undefined,
      );
      setCreatedKey(res.api_key ?? res.key ?? "");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to create API key",
      );
    } finally {
      setSubmitting(false);
    }
  };

  const copyKey = async () => {
    if (!createdKey) return;
    await navigator.clipboard.writeText(createdKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (createdKey) {
    return (
      <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-6">
        <div className="flex items-start gap-3">
          <CheckCircle2 className="mt-0.5 h-5 w-5 text-emerald-600" />
          <div className="flex-1">
            <h3 className="text-base font-semibold text-emerald-800">
              API Key Created
            </h3>
            <p className="mt-1 text-sm text-emerald-700">
              Copy this key now — it won&apos;t be shown again.
            </p>
            <div className="mt-3 flex items-center gap-2">
              <code className="flex-1 rounded-lg bg-white px-4 py-2 font-mono text-sm text-gray-900 ring-1 ring-emerald-200">
                {createdKey}
              </code>
              <button onClick={copyKey} className="btn-secondary shrink-0">
                {copied ? (
                  <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
                {copied ? "Copied!" : "Copy"}
              </button>
            </div>
            <button
              onClick={() => onSuccess(createdKey)}
              className="btn-primary mt-4"
            >
              Done
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-xl border bg-white p-6 shadow-sm">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">
          Create New API Key
        </h2>
        <button
          onClick={onClose}
          className="text-gray-400 hover:text-gray-600"
        >
          ✕
        </button>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label
            htmlFor="key-name"
            className="mb-1.5 block text-sm font-medium text-gray-700"
          >
            Name
          </label>
          <input
            id="key-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. my-app, staging, ci-pipeline"
            className="input-base"
            required
          />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label
              htmlFor="rate-limit"
              className="mb-1.5 block text-sm font-medium text-gray-700"
            >
              Rate Limit (req/min)
            </label>
            <input
              id="rate-limit"
              type="number"
              min={1}
              max={10000}
              value={rateLimit}
              onChange={(e) => setRateLimit(Number(e.target.value))}
              className="input-base"
            />
          </div>
          <div>
            <label
              htmlFor="expires-in"
              className="mb-1.5 block text-sm font-medium text-gray-700"
            >
              Expires In (optional)
            </label>
            <input
              id="expires-in"
              type="text"
              value={expiresIn}
              onChange={(e) => setExpiresIn(e.target.value)}
              placeholder="e.g. 720h, 30d"
              className="input-base"
            />
          </div>
        </div>

        {error && <ErrorAlert message={error} />}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="btn-secondary">
            Cancel
          </button>
          <button
            type="submit"
            className="btn-primary"
            disabled={submitting || !name.trim()}
          >
            {submitting ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Key className="h-4 w-4" />
            )}
            Create Key
          </button>
        </div>
      </form>
    </div>
  );
}
