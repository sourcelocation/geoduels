import bundleAnalyzer from '@next/bundle-analyzer';
import { PHASE_DEVELOPMENT_SERVER } from 'next/constants.js';
import { fileURLToPath } from 'node:url';

const appRoot = fileURLToPath(new URL('./', import.meta.url));

const withBundleAnalyzer = bundleAnalyzer({
  enabled: process.env.ANALYZE === 'true'
});

/** @type {(phase: string) => import('next').NextConfig} */
const createNextConfig = (phase) => ({
  // Keep the long-running dev server isolated from `next build`. Both commands
  // otherwise mutate `.next`, which can leave dev serving deleted manifests.
  distDir: phase === PHASE_DEVELOPMENT_SERVER ? ".next-dev" : ".next",
  reactStrictMode: true,
  output: "standalone",
  // Pages cached on the server (lib/cached-page.ts) live in memory, up to this, and nowhere else: a
  // page that falls out is built again on its next request.
  cacheMaxMemorySize: 32 * 1024 * 1024,
  experimental: { isrFlushToDisk: false },
  // Development's front door is this server: sockets go on to the gateway and the rest of /api to the
  // API, as the ingress and compose's front door route them.
  ...(phase === PHASE_DEVELOPMENT_SERVER && {
    async rewrites() {
      const api = (process.env.API_URL ??= "http://localhost:24001");
      const gateway = process.env.GATEWAY_URL ?? "http://localhost:24002";
      return {
        beforeFiles: [
          { source: "/api/live/:path*", destination: `${gateway}/api/live/:path*` },
          { source: "/api/:path*", destination: `${api}/api/:path*` },
        ],
      };
    }
  }),
  outputFileTracingRoot: appRoot,
  async headers() {
    return [
      {
        source: "/:path*.v:version(\\d+).:ext(jpg|jpeg|png|webp|avif|svg|ico|ogg|mp3|woff|woff2)",
        headers: [
          {
            key: "Cache-Control",
            value: "public, max-age=31536000, immutable"
          }
        ]
      }
    ];
  },
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "lh3.googleusercontent.com",
        pathname: "/**"
      }
    ]
  }
});

export default (phase) => withBundleAnalyzer(createNextConfig(phase));
