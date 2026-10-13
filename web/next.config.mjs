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
  // Development's front door is this server: /api goes on to the API (compose's, at GEODUELS_API),
  // its sockets included, as the ingress and compose's front door send it elsewhere.
  ...(phase === PHASE_DEVELOPMENT_SERVER && {
    async rewrites() {
      const api = process.env.GEODUELS_API ?? "http://localhost:24001";
      return { beforeFiles: [{ source: "/api/:path*", destination: `${api}/api/:path*` }] };
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
      },
      {
        // Avoid storing route documents so clients always fetch the latest
        // document after a deploy, while keeping static assets cacheable.
        source: "/((?!_next/static|_next/image|.*\\..*).*)",
        headers: [
          {
            key: "Cache-Control",
            value: "no-store"
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
