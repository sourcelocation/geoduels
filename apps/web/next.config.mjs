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
  experimental: {
    webpackBuildWorker: false
  },
  output: "standalone",
  outputFileTracingRoot: appRoot,
  async headers() {
    return [
      {
        source: "/runtime-config.js",
        headers: [
          {
            key: "Cache-Control",
            value: "no-store"
          }
        ]
      },
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
