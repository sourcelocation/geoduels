import { useRouter } from "next/router";
import { getLobbyLayout } from "../../features/home/page/LobbyApplicationLayout";
import LobbyRoutePage from "../../features/home/page/LobbyRoutePage";
import { buildMapSocialPreview } from "../../features/maps/lib/map-social-preview";
import {
  normalizeEntityRouteId,
  toPublicEntityId,
} from "../../lib/entity-id";
import { cachedPage, onDemandPaths, routeParam } from "../../lib/cached-page";
import { loadPublicMap } from "../../lib/public-data";
import { mapDetailsKey } from "../../features/maps/lib/map-hooks";
import type { SocialPreview } from "../../lib/social-preview";
import type { NextPageWithLayout } from "../_app";

type MapDetailsRouteProps = {
  preview: SocialPreview;
};

export const getStaticPaths = onDemandPaths;

export const getStaticProps = cachedPage<MapDetailsRouteProps>(async (context, queries) => {
  const routeId = routeParam(context, "id");
  const mapId = normalizeEntityRouteId(routeId);
  const details = await loadPublicMap(mapId);
  if (details) queries.setQueryData(mapDetailsKey(mapId, ""), details);
  return {
    preview: buildMapSocialPreview(details?.map || null, details?.countryStats || null, routeId),
  };
});

const MapDetailsRoute: NextPageWithLayout<MapDetailsRouteProps> = function MapDetailsRoute({
  preview,
}) {
  const router = useRouter();
  const mapId = typeof router.query.id === "string" ? normalizeEntityRouteId(router.query.id) : "";

  return (
    <LobbyRoutePage
      title={preview.title}
      description={preview.description}
      canonicalPath={
        mapId
          ? `/maps/${encodeURIComponent(toPublicEntityId(mapId))}`
          : preview.canonicalPath
      }
      preview={preview}
    />
  );
};

MapDetailsRoute.getLayout = getLobbyLayout;

export default MapDetailsRoute;
