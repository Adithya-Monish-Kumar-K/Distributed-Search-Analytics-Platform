import type { Metadata } from "next";
import "./globals.css";
import Sidebar from "@/components/sidebar";

export const metadata: Metadata = {
  title: "Search Platform",
  description: "Distributed Search & Analytics Platform",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>
        <Sidebar />
        <main className="min-h-screen bg-gray-50 lg:pl-64">
          <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
            {children}
          </div>
        </main>
      </body>
    </html>
  );
}
