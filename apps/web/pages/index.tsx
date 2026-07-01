import Head from "next/head";
import { getLobbyLayout } from "../features/home/page/LobbyApplicationLayout";
import { getSiteURL } from "../lib/site";
import type { NextPageWithLayout } from "./_app";

const HomePage: NextPageWithLayout = function HomePage() {
  const siteURL = getSiteURL();
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
