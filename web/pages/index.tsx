import Head from "next/head";
import { getLobbyLayout } from "../features/home/page/LobbyApplicationLayout";
import { useSiteURL } from "../lib/site";
import type { NextPageWithLayout } from "./_app";
import { mapListKey } from "../features/maps/lib/map-hooks";
import { listMaps } from "../features/maps/lib/maps-client";
import { cachedPage } from "../lib/cached-page";

export const getStaticProps = cachedPage(async (_context, queries) => {
  const [official, awarded] = await Promise.all([
    listMaps(undefined, { scope: "official" }),
    listMaps(undefined, { scope: "awarded" }),
  ]);
  queries.setQueryData(mapListKey("official", "trending", "", ""), official);
  queries.setQueryData(mapListKey("awarded", "trending", "", ""), awarded);
  return {};
});

const HomePage: NextPageWithLayout = function HomePage() {
  const siteURL = useSiteURL();
  const canonicalURL = `${siteURL}/`;
  const title = "GeoDuels | Play";
  const description =
    "Play the best free GeoGuessr alternative with ranked duels, online 1v1 games, singleplayer, or 2v2 with friends!";

  return (
    <Head>
      <title>{title}</title>
      <meta name="description" content={description} />
      <meta name="robots" content="index,follow" />
      <link rel="canonical" href={canonicalURL} />
      <meta property="og:type" content="website" />
      <meta property="og:site_name" content="GeoDuels" />
      <meta property="og:title" content={title} />
      <meta property="og:description" content={description} />
      <meta property="og:url" content={canonicalURL} />
      <meta property="og:image" content={`${siteURL}/logo.v2.png`} />
      <meta name="twitter:card" content="summary_large_image" />
      <meta name="twitter:title" content={title} />
      <meta name="twitter:description" content={description} />
      <meta name="twitter:image" content={`${siteURL}/logo.v2.png`} />
    </Head>
  );
};

HomePage.getLayout = getLobbyLayout;

export default HomePage;
