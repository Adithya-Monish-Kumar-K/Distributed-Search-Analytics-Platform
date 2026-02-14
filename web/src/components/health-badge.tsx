"use client";

import { cn } from "@/lib/utils";
import type { ServiceStatus } from "@/lib/types";

interface HealthBadgeProps {
  status: ServiceStatus;
  label: string;
  className?: string;
}

const statusStyles: Record<ServiceStatus, string> = {
  healthy: "badge-green",
  unhealthy: "badge-red",
  unknown: "badge-gray",
};

const statusDot: Record<ServiceStatus, string> = {
  healthy: "bg-emerald-500",
  unhealthy: "bg-red-500",
  unknown: "bg-gray-400",
};

export default function HealthBadge({
  status,
  label,
  className,
}: HealthBadgeProps) {
  return (
    <span className={cn(statusStyles[status], className)}>
      <span
        className={cn("mr-1.5 inline-block h-2 w-2 rounded-full", statusDot[status])}
      />
      {label}
    </span>
  );
}
