import LobbyRoutePage from "../features/home/page/LobbyRoutePage";
import { getLobbyLayout } from "../features/home/page/LobbyApplicationLayout";
import type { NextPageWithLayout } from "./_app";

const TopPage: NextPageWithLayout = function TopPage() {
  return (
    <LobbyRoutePage
      title="GeoDuels | Leaderboard"
      description="Browse the GeoDuels ranked leaderboard."
      canonicalPath="/top"
    />
  );
};

TopPage.getLayout = getLobbyLayout;

export default TopPage;
