import Head from "next/head";
import MarkdownContent from "../../components/ui/MarkdownContent";
import Link from "next/link";
import { DocumentPage } from "../../features/app-shell/components/DocumentPage";
import { requestChangelogPost } from "../../features/changelog/changelog-client";
import type { ChangelogPost } from "../../features/changelog/types";
import { useSiteURL } from "../../lib/site";
import { cachedPage, onDemandPaths, routeParam } from "../../lib/cached-page";

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

export const getStaticPaths = onDemandPaths;

export const getStaticProps = cachedPage<ChangelogPostPageProps>(async (context) => {
  const post = await requestChangelogPost(routeParam(context, "slug"));
  return post && { post };
});

export default function ChangelogPostPage({ post }: ChangelogPostPageProps) {
  const siteURL = useSiteURL();
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
      <DocumentPage>
        <Link href="/changelog" className="text-body-sm font-strong text-content-secondary transition hover:text-content-primary">Changelog</Link>
        <article>
          <header className="mt-6 border-b border-border-default pb-8">
            <time dateTime={post.updatedAt} className="text-label font-strong text-status-success">
              Updated {formatDate(post.updatedAt)}
            </time>
            <h1 className="mt-3 text-display-md font-strong tracking-heading text-content-primary">
              {post.title}
            </h1>
          </header>
          <MarkdownContent markdown={post.markdown} className="mt-8" />
        </article>
      </DocumentPage>
    </>
  );
}
