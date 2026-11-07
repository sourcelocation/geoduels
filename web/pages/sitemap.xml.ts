import type { GetServerSideProps } from "next";
import { requestLeaderboard } from "../features/lobby/lib/leaderboard";
import { requestChangelogPosts } from "../features/changelog/changelog-client";
import { listMaps } from "../features/maps/lib/maps-client";
import { siteURL } from "../lib/cached-page";
import { toPublicEntityId } from "../lib/entity-id";

const CACHE_CONTROL = "public, s-maxage=3600, stale-while-revalidate=86400";

export const getServerSideProps: GetServerSideProps = async ({ res }) => {
  const site = siteURL();
  try {
    const [posts, officialMaps, communityMaps, leaderboard] = await Promise.all([
      requestChangelogPosts(),
      listMaps(undefined, { scope: "official" }),
      listMaps(undefined, { scope: "community", sort: "popular" }),
      requestLeaderboard(),
    ]);
    const urls: Array<{ loc: string; lastmod?: string }> = [
      { loc: `${site}/` },
      { loc: `${site}/maps` },
      { loc: `${site}/help` },
      { loc: `${site}/changelog` },
      ...(posts.posts || []).map((post) => ({
        loc: `${site}/changelog/${encodeURIComponent(post.slug)}`,
        lastmod: post.updatedAt,
      })),
      ...[...officialMaps, ...communityMaps]
        .filter((map) => map.status === "ready")
        .map((map) => ({ loc: `${site}/maps/${encodeURIComponent(toPublicEntityId(map.id))}` })),
      ...(leaderboard?.entries || []).map((entry) => ({
        loc: `${site}/players/${encodeURIComponent(entry.displayName)}`,
      })),
      { loc: `${site}/privacy` },
      { loc: `${site}/privacy/extension` },
      { loc: `${site}/terms` },
    ];
    res.setHeader("Content-Type", "application/xml");
    res.setHeader("Cache-Control", CACHE_CONTROL);
    res.write(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls
  .map((url) => `  <url><loc>${url.loc}</loc>${url.lastmod ? `<lastmod>${url.lastmod}</lastmod>` : ""}</url>`)
  .join("\n")}
</urlset>`);
  } catch {
    // Search engines keep the last sitemap they read; a shorter one would tell them pages went away.
    res.statusCode = 503;
    res.setHeader("Retry-After", "300");
  }
  res.end();
  return { props: {} };
};

export default function Sitemap() {
  return null;
}
