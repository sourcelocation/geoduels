import type { GetServerSideProps } from "next";
import Head from "next/head";
import Link from "next/link";
import { EmptyState } from "../../components/ui/EmptyState";
import MarkdownContent from "../../components/ui/MarkdownContent";
import { PageShell } from "../../components/ui/PageShell";
import { DocumentPanel } from "../../components/ui/compositions";
import { requestChangelogPosts } from "../../features/changelog/changelog-client";
import type { ChangelogPost } from "../../features/changelog/types";
import { createRuntimeConfig } from "../../lib/runtime-config";
import { getSiteURL } from "../../lib/site";

type ChangelogIndexProps = {
  posts: ChangelogPost[];
};

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("en-US", {
    month: "long",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  }).format(date);
}

export const getServerSideProps: GetServerSideProps<ChangelogIndexProps> = async () => {
  try {
    const data = await requestChangelogPosts(createRuntimeConfig());
    return { props: { posts: data.posts || [] } };
  } catch {
    return { props: { posts: [] } };
  }
};

export default function ChangelogIndexPage({ posts }: ChangelogIndexProps) {
  const siteURL = getSiteURL();
  const title = "GeoDuels Changelog";
  const description =
    "Read GeoDuels release notes, gameplay updates, new features, and fixes.";

  return (
    <>
      <Head>
        <title>{title}</title>
        <meta name="description" content={description} />
        <meta name="robots" content="index,follow" />
        <link rel="canonical" href={`${siteURL}/changelog`} />
        <meta property="og:type" content="website" />
        <meta property="og:title" content={title} />
        <meta property="og:description" content={description} />
        <meta property="og:url" content={`${siteURL}/changelog`} />
      </Head>
      <PageShell
        variant="operational"
        eyebrow="Updates"
        title="GeoDuels Changelog"
        backHref="/"
      >
          <div className="mt-10 space-y-4">
            {posts.map((post) => (
              <DocumentPanel key={post.id} as="article" className="p-5 sm:p-6">
                <time dateTime={post.updatedAt} className="text-label font-strong text-status-success">
                  {formatDate(post.updatedAt)}
                </time>
                <h2 className="mt-2 text-heading-lg font-strong text-content-primary">
                  <Link href={`/changelog/${encodeURIComponent(post.slug)}`} className="hover:text-status-success">
                    {post.title}
                  </Link>
                </h2>
                <div className="mt-3 max-h-32 overflow-hidden [mask-image:linear-gradient(180deg,black_62%,transparent_100%)] [-webkit-mask-image:linear-gradient(180deg,black_62%,transparent_100%)]">
                  <MarkdownContent markdown={post.markdown || "No changelog content yet."} compact />
                </div>
              </DocumentPanel>
            ))}
            {posts.length === 0 ? (
              <EmptyState message="No changelog posts have been published yet." />
            ) : null}
          </div>
      </PageShell>
    </>
  );
}
