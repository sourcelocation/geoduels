import Head from "next/head";
import { getSiteURL } from "../../../lib/site";

type LobbyRoutePageProps = {
  title: string;
  description: string;
  canonicalPath: string;
};

export default function LobbyRoutePage({
  title,
  description,
  canonicalPath,
}: LobbyRoutePageProps) {
  const siteURL = getSiteURL();
  const canonicalURL = `${siteURL}${canonicalPath}`;

  return (
    <>
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
    </>
  );
}
