import LobbyRoutePage from "../../features/home/page/LobbyRoutePage";
import { getLobbyLayout } from "../../features/home/page/LobbyApplicationLayout";
import type { NextPageWithLayout } from "../_app";
import { mapListKey } from "../../features/maps/lib/map-hooks";
import { listMaps } from "../../features/maps/lib/maps-client";
import { cachedPage } from "../../lib/cached-page";

export const getStaticProps = cachedPage(async (_context, queries) => {
  const maps = await listMaps(undefined, { scope: "community", sort: "trending" });
  queries.setQueryData(mapListKey("community", "trending", "", ""), maps);
  return {};
});

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
