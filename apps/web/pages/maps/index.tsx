import LobbyRoutePage from "../../features/home/page/LobbyRoutePage";
import { getLobbyLayout } from "../../features/home/page/LobbyApplicationLayout";
import type { NextPageWithLayout } from "../_app";

const MapsPage: NextPageWithLayout = function MapsPage() {
  return (
    <LobbyRoutePage
      title="GeoDuels | Maps"
      description="Browse official and community GeoDuels maps."
      canonicalPath="/maps"
    />
  );
};

MapsPage.getLayout = getLobbyLayout;

export default MapsPage;
