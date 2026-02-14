"use client";

import { AlertTriangle } from "lucide-react";

interface ErrorAlertProps {
  title?: string;
  message: string;
  onRetry?: () => void;
}

export default function ErrorAlert({
  title = "Error",
  message,
  onRetry,
}: ErrorAlertProps) {
  return (
    <div className="rounded-lg border border-red-200 bg-red-50 p-4">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-red-600" />
        <div className="flex-1">
          <h3 className="text-sm font-semibold text-red-800">{title}</h3>
          <p className="mt-1 text-sm text-red-700">{message}</p>
          {onRetry && (
            <button
              onClick={onRetry}
              className="mt-3 text-sm font-medium text-red-700 underline hover:text-red-900"
            >
              Try again
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
