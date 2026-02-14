"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Search,
  LayoutDashboard,
  FileText,
  BarChart3,
  Key,
  Database,
  Activity,
  Menu,
  X,
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";

const navItems = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/search", label: "Search", icon: Search },
  { href: "/documents", label: "Documents", icon: FileText },
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/api-keys", label: "API Keys", icon: Key },
  { href: "/cache", label: "Cache", icon: Database },
];

export default function Sidebar() {
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <>
      {/* Mobile toggle */}
      <button
        onClick={() => setMobileOpen(true)}
        className="fixed left-4 top-4 z-50 rounded-lg bg-gray-900 p-2 text-white lg:hidden"
        aria-label="Open menu"
      >
        <Menu className="h-5 w-5" />
      </button>

      {/* Backdrop */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-50 flex w-64 flex-col bg-gray-950 transition-transform lg:translate-x-0",
          mobileOpen ? "translate-x-0" : "-translate-x-full",
        )}
      >
        {/* Logo */}
        <div className="flex h-16 items-center justify-between px-6">
          <Link href="/" className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-600">
              <Activity className="h-4 w-4 text-white" />
            </div>
            <span className="text-lg font-semibold text-white">
              SearchPlatform
            </span>
          </Link>
          <button
            className="rounded p-1 text-gray-400 hover:text-white lg:hidden"
            onClick={() => setMobileOpen(false)}
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Nav */}
        <nav className="flex-1 space-y-1 px-3 py-4">
          {navItems.map(({ href, label, icon: Icon }) => {
            const active =
              href === "/" ? pathname === "/" : pathname.startsWith(href);
            return (
              <Link
                key={href}
                href={href}
                onClick={() => setMobileOpen(false)}
                className={cn("sidebar-link", active && "sidebar-link-active")}
              >
                <Icon className="h-5 w-5 shrink-0" />
                {label}
              </Link>
            );
          })}
        </nav>

        {/* Footer */}
        <div className="border-t border-white/10 px-6 py-4">
          <p className="text-xs text-slate-500">
            Distributed Search &amp; Analytics Platform
          </p>
          <p className="mt-0.5 text-xs text-slate-600">v1.0.0</p>
        </div>
      </aside>
    </>
  );
}
