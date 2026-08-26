import type { GetServerSideProps } from "next";
import Head from "next/head";
import MarkdownContent from "../../components/ui/MarkdownContent";
import { PageShell } from "../../components/ui/PageShell";
import { requestChangelogPost } from "../../features/changelog/changelog-client";
import type { ChangelogPost } from "../../features/changelog/types";
import { createRuntimeConfig } from "../../lib/runtime-config";
import { getSiteURL } from "../../lib/site";

type ChangelogPostPageProps = {
  post: ChangelogPost;
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

export const getServerSideProps: GetServerSideProps<ChangelogPostPageProps> = async (ctx) => {
  const slug = typeof ctx.params?.slug === "string" ? ctx.params.slug : "";
  if (!slug) return { notFound: true };
  try {
    const post = await requestChangelogPost(createRuntimeConfig(), slug);
    if (!post) return { notFound: true };
    ctx.res.setHeader("Last-Modified", new Date(post.updatedAt).toUTCString());
    return { props: { post } };
  } catch {
    return { notFound: true };
  }
};

export default function ChangelogPostPage({ post }: ChangelogPostPageProps) {
  const siteURL = getSiteURL();
  const canonicalURL = `${siteURL}/changelog/${post.slug}`;
  const description = `Read the ${post.title} update notes for GeoDuels.`;

  return (
    <>
      <Head>
        <title>{post.title} | GeoDuels Changelog</title>
        <meta name="description" content={description} />
        <meta name="robots" content="index,follow" />
        <link rel="canonical" href={canonicalURL} />
        <meta property="og:type" content="article" />
        <meta property="og:title" content={post.title} />
        <meta property="og:description" content={description} />
        <meta property="og:url" content={canonicalURL} />
        <meta property="article:modified_time" content={post.updatedAt} />
      </Head>
      <PageShell variant="operational" backHref="/changelog" backLabel="Changelog" maxWidthClassName="max-w-3xl">
        <article>
          <header className="border-b border-border-default pb-8">
            <time dateTime={post.updatedAt} className="text-label font-strong text-status-success">
              Updated {formatDate(post.updatedAt)}
            </time>
            <h1 className="mt-3 text-display-md font-strong tracking-heading text-content-primary">
              {post.title}
            </h1>
          </header>
          <MarkdownContent markdown={post.markdown} className="mt-8" />
        </article>
      </PageShell>
    </>
  );
}
