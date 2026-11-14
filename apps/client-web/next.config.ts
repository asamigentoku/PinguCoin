import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const bffApiUrl = (process.env.BFF_API_URL ?? "http://localhost:8082/api/v1").replace(/\/$/, "");

const nextConfig: NextConfig = {
  async rewrites() {
    return [{ source: "/api/v1/:path*", destination: `${bffApiUrl}/:path*` }];
  },
};

const withNextIntl = createNextIntlPlugin();
export default withNextIntl(nextConfig);
