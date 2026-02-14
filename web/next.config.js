/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    const gateway = process.env.NEXT_PUBLIC_GATEWAY_URL || "http://localhost:8082";
    const search = process.env.NEXT_PUBLIC_SEARCH_URL || "http://localhost:8080";
    const ingestion = process.env.NEXT_PUBLIC_INGESTION_URL || "http://localhost:8081";
    return [
      { source: "/api/gateway/:path*", destination: `${gateway}/:path*` },
      { source: "/api/search/:path*", destination: `${search}/:path*` },
      { source: "/api/ingest/:path*", destination: `${ingestion}/:path*` },
    ];
  },
};

module.exports = nextConfig;
