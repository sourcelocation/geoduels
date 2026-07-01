import Head from "next/head";
import { useRouter } from "next/router";
import { getLobbyLayout } from "../../features/home/page/LobbyApplicationLayout";
import { getSiteURL } from "../../lib/site";
import type { NextPageWithLayout } from "../_app";

const PartyInviteRoute: NextPageWithLayout = function PartyInviteRoute() {
  const router = useRouter();
  const rawCode =
    router.isReady && typeof router.query.code === "string"
      ? router.query.code
      : "";
  const partyInviteCode = rawCode.trim().toUpperCase();
  const siteURL = getSiteURL();
  const canonicalURL = partyInviteCode
    ? `${siteURL}/party/${encodeURIComponent(partyInviteCode)}`
    : `${siteURL}/`;
  const title = partyInviteCode
    ? `GeoDuels | Party ${partyInviteCode}`
    : "GeoDuels | Party";
  const description =
    "Join a private GeoDuels party, invite a friend or guest, and start a duel together.";

  return (
    <Head>
      <title>{title}</title>
      <meta name="description" content={description} />
      <meta name="robots" content="noindex,nofollow" />
      <link rel="canonical" href={canonicalURL} />
      <meta property="og:type" content="website" />
      <meta property="og:site_name" content="GeoDuels" />
      <meta property="og:title" content={title} />
      <meta property="og:description" content={description} />
      <meta property="og:url" content={canonicalURL} />
      <meta property="og:image" content={`${siteURL}/logo.v2.png`} />
    </Head>
  );
};

PartyInviteRoute.getLayout = getLobbyLayout;

export default PartyInviteRoute;
