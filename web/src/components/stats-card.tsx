"use client";

import { cn, formatNumber } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";

interface StatsCardProps {
  title: string;
  value: string | number;
  subtitle?: string;
  icon: LucideIcon;
  trend?: { value: number; label: string };
  color?: "brand" | "emerald" | "amber" | "red" | "blue";
}

const colorMap = {
  brand: "bg-brand-50 text-brand-600",
  emerald: "bg-emerald-50 text-emerald-600",
  amber: "bg-amber-50 text-amber-600",
  red: "bg-red-50 text-red-600",
  blue: "bg-blue-50 text-blue-600",
};

export default function StatsCard({
  title,
  value,
  subtitle,
  icon: Icon,
  trend,
  color = "brand",
}: StatsCardProps) {
  const displayValue =
    typeof value === "number" ? formatNumber(value) : value;

  return (
    <div className="stat-card">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-medium text-gray-500">{title}</p>
          <p className="mt-2 text-3xl font-bold tracking-tight text-gray-900">
            {displayValue}
          </p>
          {subtitle && (
            <p className="mt-1 text-sm text-gray-500">{subtitle}</p>
          )}
          {trend && (
            <p
              className={cn(
                "mt-2 text-sm font-medium",
                trend.value >= 0 ? "text-emerald-600" : "text-red-600",
              )}
            >
              {trend.value >= 0 ? "↑" : "↓"} {Math.abs(trend.value)}%{" "}
              <span className="font-normal text-gray-500">{trend.label}</span>
            </p>
          )}
        </div>
        <div className={cn("rounded-lg p-3", colorMap[color])}>
          <Icon className="h-6 w-6" />
        </div>
      </div>
    </div>
  );
}
