import { useRouter } from "next/router";
import { PlayerProfilePage } from "../../features/players/components/PlayerProfilePage";
import type { PublicPlayerProfile } from "../../features/players/types";
import { cachedPage, onDemandPaths, routeParam } from "../../lib/cached-page";
import { loadPublicProfile } from "../../lib/public-data";

type PlayerRouteProps = {
  playerId: string;
  initialProfile: PublicPlayerProfile;
};

export const getStaticPaths = onDemandPaths;

export const getStaticProps = cachedPage<PlayerRouteProps>(async (context) => {
  const playerId = routeParam(context, "id");
  const initialProfile = await loadPublicProfile(playerId);
  return initialProfile && { playerId, initialProfile };
});

export default function PlayerRoute({
  playerId,
  initialProfile,
}: PlayerRouteProps) {
  const router = useRouter();
  const routedId = typeof router.query.id === "string" ? router.query.id.trim() : "";
  return (
    <PlayerProfilePage
      playerId={routedId || playerId}
      initialProfile={initialProfile}
    />
  );
}
