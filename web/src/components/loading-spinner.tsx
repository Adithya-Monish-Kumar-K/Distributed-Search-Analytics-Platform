"use client";

import { Loader2 } from "lucide-react";

interface LoadingSpinnerProps {
  message?: string;
}

export default function LoadingSpinner({
  message = "Loading...",
}: LoadingSpinnerProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <Loader2 className="h-8 w-8 animate-spin text-brand-600" />
      <p className="mt-3 text-sm text-gray-500">{message}</p>
    </div>
  );
}
