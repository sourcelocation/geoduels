import LobbyRoutePage from "../features/home/page/LobbyRoutePage";
import { getLobbyLayout } from "../features/home/page/LobbyApplicationLayout";
import type { NextPageWithLayout } from "./_app";
import { cachedPage } from "../lib/cached-page";

export const getStaticProps = cachedPage();

const FriendsPage: NextPageWithLayout = function FriendsPage() {
  return (
    <LobbyRoutePage
      title="GeoDuels | Friends"
      description="Create or join a private GeoDuels party and play with friends."
      canonicalPath="/friends"
    />
  );
};

FriendsPage.getLayout = getLobbyLayout;

export default FriendsPage;
