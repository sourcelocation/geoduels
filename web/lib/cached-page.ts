import { QueryClient, dehydrate, type DehydratedState } from "@tanstack/react-query";
import type { GetStaticPaths, GetStaticProps, GetStaticPropsContext } from "next";

// How long a page is served before the next request rebuilds it in the background. If rebuilding
// fails, because the API is down, the last good page keeps being served.
const REVALIDATE_SECONDS = 60;
// No API while the image builds: the page is built without data and rebuilt on its first request.
const REBUILD_NOW_SECONDS = 1;

export function siteURL() {
  return (process.env.SITE_URL || "http://localhost:24000").replace(/\/$/, "");
}

export type CachedPageProps = { siteURL: string; dehydratedState: DehydratedState };

type Load<P> = (context: GetStaticPropsContext, queries: QueryClient) => Promise<P | null>;

/**
 * A page anyone may see: public data only, built on the server and served from its cache. What load
 * puts in `queries` is what the page's own useQuery calls find, so the normal interface renders with
 * real content, and the browser takes over from there. A null from load is a 404.
 */
export function cachedPage<P extends Record<string, unknown> = Record<string, never>>(
  load?: Load<P>,
  withoutAPI?: P,
): GetStaticProps<P & CachedPageProps> {
  return async (context) => {
    const queries = new QueryClient();
    const building = !process.env.API_URL;
    const props = building || !load ? (withoutAPI ?? ({} as P)) : await load(context, queries);
    if (props === null) return { notFound: true, revalidate: REVALIDATE_SECONDS };
    return {
      props: { ...props, siteURL: siteURL(), dehydratedState: dehydrate(queries) },
      revalidate: building ? REBUILD_NOW_SECONDS : REVALIDATE_SECONDS,
    };
  };
}

/** For a page whose paths aren't known ahead: each is built on its first request, then cached. */
export const onDemandPaths: GetStaticPaths = async () => ({ paths: [], fallback: "blocking" });

export function routeParam(context: GetStaticPropsContext, name: string) {
  const value = context.params?.[name];
  return typeof value === "string" ? value.trim() : "";
}
