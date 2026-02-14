"use client";

import { useState, useCallback, useEffect } from "react";
import {
  FileText,
  Plus,
  RefreshCcw,
  Upload,
  CheckCircle2,
  XCircle,
  Clock,
  Loader2,
} from "lucide-react";
import { ingestDocument, getDocuments } from "@/lib/api";
import type { Document } from "@/lib/types";
import LoadingSpinner from "@/components/loading-spinner";
import ErrorAlert from "@/components/error-alert";
import EmptyState from "@/components/empty-state";
import { timeAgo } from "@/lib/utils";

const statusStyles: Record<string, string> = {
  PENDING: "badge-yellow",
  INDEXING: "badge-blue",
  INDEXED: "badge-green",
  FAILED: "badge-red",
  DELETED: "badge-gray",
};

const statusIcons: Record<string, React.ReactNode> = {
  PENDING: <Clock className="h-3 w-3" />,
  INDEXING: <Loader2 className="h-3 w-3 animate-spin" />,
  INDEXED: <CheckCircle2 className="h-3 w-3" />,
  FAILED: <XCircle className="h-3 w-3" />,
  DELETED: <XCircle className="h-3 w-3" />,
};

export default function DocumentsPage() {
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showIngest, setShowIngest] = useState(false);

  // Read the stored gateway API key from localStorage.
  const getStoredApiKey = () =>
    (typeof window !== "undefined" && localStorage.getItem("sp_api_key")) || undefined;

  const fetchDocs = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await getDocuments(getStoredApiKey());
      setDocuments(res.documents ?? []);
    } catch (err) {
      // Some service configurations may not expose the documents list endpoint
      setError(
        err instanceof Error
          ? err.message
          : "Failed to fetch documents",
      );
      setDocuments([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchDocs();
  }, [fetchDocs]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Documents</h1>
          <p className="mt-1 text-sm text-gray-500">
            Ingest and manage documents in the search index
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={fetchDocs}
            className="btn-secondary"
            disabled={loading}
          >
            <RefreshCcw
              className={`h-4 w-4 ${loading ? "animate-spin" : ""}`}
            />
            Refresh
          </button>
          <button
            onClick={() => setShowIngest(true)}
            className="btn-primary"
          >
            <Plus className="h-4 w-4" />
            Ingest Document
          </button>
        </div>
      </div>

      {error && <ErrorAlert message={error} onRetry={fetchDocs} />}

      {/* Ingest Form */}
      {showIngest && (
        <IngestForm
          onClose={() => setShowIngest(false)}
          onSuccess={() => {
            setShowIngest(false);
            setTimeout(fetchDocs, 1000); // Allow ingestion to process
          }}
        />
      )}

      {/* Document List */}
      {loading && documents.length === 0 ? (
        <LoadingSpinner message="Loading documents..." />
      ) : documents.length === 0 && !error ? (
        <EmptyState
          icon={<FileText className="h-12 w-12" />}
          title="No documents yet"
          description="Ingest your first document to get started with the search platform."
          action={
            <button
              onClick={() => setShowIngest(true)}
              className="btn-primary"
            >
              <Upload className="h-4 w-4" />
              Ingest Document
            </button>
          }
        />
      ) : (
        <div className="overflow-hidden rounded-xl border bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b bg-gray-50">
                <th className="px-6 py-3 font-medium text-gray-500">Title</th>
                <th className="px-6 py-3 font-medium text-gray-500">
                  Status
                </th>
                <th className="px-6 py-3 font-medium text-gray-500">Shard</th>
                <th className="px-6 py-3 font-medium text-gray-500">Size</th>
                <th className="px-6 py-3 font-medium text-gray-500">
                  Created
                </th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {documents.map((doc) => (
                <tr key={doc.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <div>
                      <p className="font-medium text-gray-900">{doc.title}</p>
                      <p className="mt-0.5 font-mono text-xs text-gray-400">
                        {doc.id.slice(0, 8)}…
                      </p>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className={statusStyles[doc.status] ?? "badge-gray"}>
                      <span className="mr-1">{statusIcons[doc.status]}</span>
                      {doc.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-gray-600">{doc.shard_id}</td>
                  <td className="px-6 py-4 text-gray-600">
                    {doc.content_size} B
                  </td>
                  <td className="px-6 py-4 text-gray-500">
                    {timeAgo(doc.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// ─── Ingest Form ────────────────────────────────────────────

function IngestForm({
  onClose,
  onSuccess,
}: {
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() || !body.trim()) return;

    setSubmitting(true);
    setError(null);
    setSuccess(null);

    try {
      const res = await ingestDocument({ title: title.trim(), body: body.trim() });
      setSuccess(`Document ingested successfully (ID: ${res.id})`);
      setTitle("");
      setBody("");
      setTimeout(onSuccess, 1500);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to ingest document",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="rounded-xl border bg-white p-6 shadow-sm">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">
          Ingest New Document
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
            htmlFor="doc-title"
            className="mb-1.5 block text-sm font-medium text-gray-700"
          >
            Title
          </label>
          <input
            id="doc-title"
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Document title"
            className="input-base"
            required
          />
        </div>
        <div>
          <label
            htmlFor="doc-body"
            className="mb-1.5 block text-sm font-medium text-gray-700"
          >
            Body
          </label>
          <textarea
            id="doc-body"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Document content..."
            rows={6}
            className="input-base resize-y"
            required
          />
        </div>

        {error && <ErrorAlert message={error} />}
        {success && (
          <div className="flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
            <CheckCircle2 className="h-4 w-4" />
            {success}
          </div>
        )}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="btn-secondary">
            Cancel
          </button>
          <button
            type="submit"
            className="btn-primary"
            disabled={submitting || !title.trim() || !body.trim()}
          >
            {submitting ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Upload className="h-4 w-4" />
            )}
            Ingest
          </button>
        </div>
      </form>
    </div>
  );
}
