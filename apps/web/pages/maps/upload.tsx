import LobbyRoutePage from "../../features/home/page/LobbyRoutePage";
import { getLobbyLayout } from "../../features/home/page/LobbyApplicationLayout";
import type { NextPageWithLayout } from "../_app";

const MapUploadRoute: NextPageWithLayout = function MapUploadRoute() {
  return (
    <LobbyRoutePage
      title="GeoDuels | Upload Map"
      description="Upload a custom GeoDuels map."
      canonicalPath="/maps/upload"
    />
  );
};

MapUploadRoute.getLayout = getLobbyLayout;

export default MapUploadRoute;
