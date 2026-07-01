import type { GetServerSideProps } from 'next';
import { requestChangelogPosts } from '../features/changelog/changelog-client';
import { createRuntimeConfig } from '../lib/runtime-config';
import { getSiteURL } from '../lib/site';

export const getServerSideProps: GetServerSideProps = async ({ res }) => {
  const siteURL = getSiteURL();
  const now = new Date().toISOString();
  let changelogPosts: Array<{ slug: string; updatedAt: string }> = [];
  try {
    const data = await requestChangelogPosts(createRuntimeConfig());
    changelogPosts = data.posts || [];
  } catch {
    changelogPosts = [];
  }
  const urls: Array<{ loc: string; priority: string; lastmod?: string }> = [
    { loc: `${siteURL}/`, priority: '1.0' },
    { loc: `${siteURL}/help`, priority: '0.7' },
    { loc: `${siteURL}/changelog`, priority: '0.6' },
    ...changelogPosts.map((post) => ({
      loc: `${siteURL}/changelog/${post.slug}`,
      priority: '0.5',
      lastmod: post.updatedAt || now
    })),
    { loc: `${siteURL}/privacy`, priority: '0.3' },
    { loc: `${siteURL}/privacy/extension`, priority: '0.3' },
    { loc: `${siteURL}/terms`, priority: '0.3' }
  ];

  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls
  .map(
    (url) => `  <url>
    <loc>${url.loc}</loc>
    <lastmod>${url.lastmod || now}</lastmod>
    <changefreq>weekly</changefreq>
    <priority>${url.priority}</priority>
  </url>`
  )
  .join('\n')}
</urlset>`;

  res.setHeader('Content-Type', 'application/xml');
  res.write(xml);
  res.end();

  return {
    props: {}
  };
};

export default function SitemapXML() {
  return null;
}
