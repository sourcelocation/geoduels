import type { GetServerSideProps } from "next";
import { PlayerProfilePage } from "../../features/players/components/PlayerProfilePage";
import { requestPlayerProfile } from "../../features/players/lib/player-client";
import type { PublicPlayerProfile } from "../../features/players/types";
import { createRuntimeConfig } from "../../lib/runtime-config";

type PlayerRouteProps = {
  playerId: string;
  initialProfile: PublicPlayerProfile;
};

export default function PlayerRoute({ playerId, initialProfile }: PlayerRouteProps) {
  return <PlayerProfilePage playerId={playerId} initialProfile={initialProfile} />;
}

export const getServerSideProps: GetServerSideProps<PlayerRouteProps> = async ({ params }) => {
  const playerId = typeof params?.id === "string" ? params.id.trim() : "";
  if (!playerId) return { notFound: true };
  try {
    const initialProfile = await requestPlayerProfile(createRuntimeConfig(), playerId);
    if (playerId !== initialProfile.displayName) {
      return {
        redirect: {
          destination: `/players/${encodeURIComponent(initialProfile.displayName)}`,
          permanent: true,
        },
      };
    }
    return { props: { playerId, initialProfile } };
  } catch {
    return {
      notFound: true,
    };
  }
};
